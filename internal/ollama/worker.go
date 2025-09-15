package ollama

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Worker manages the background processing of Ollama model downloads
type Worker struct {
	db            *sql.DB
	jobQueue      chan DownloadJob
	updates       chan ProgressUpdate
	stopCh        chan struct{}
	wg            sync.WaitGroup
	containerName string
}

// NewWorker creates a new worker instance
func NewWorker(db *sql.DB) *Worker {
	return &Worker{
		db:            db,
		jobQueue:      make(chan DownloadJob, 100),
		updates:       make(chan ProgressUpdate, 1000),
		stopCh:        make(chan struct{}),
		containerName: "", // Will be determined dynamically
	}
}

// Start initializes and starts the worker pool
func (w *Worker) Start(numWorkers int) {
	log.Printf("Starting Ollama worker pool with %d workers", numWorkers)

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		w.wg.Add(1)
		go w.processJobs(i)
	}

	// Start job recovery on startup
	go w.recoverPendingJobs()

	// Start periodic cleanup
	go w.startCleanupTask()
}

// Stop gracefully shuts down the worker pool
func (w *Worker) Stop() {
	log.Println("Stopping Ollama worker pool")
	close(w.stopCh)
	w.wg.Wait()
	close(w.jobQueue)
	close(w.updates)
}

// AddJob adds a new download job to the queue
func (w *Worker) AddJob(job DownloadJob) {
	select {
	case w.jobQueue <- job:
		log.Printf("Added job %s for model %s to queue", job.ID, job.ModelName)
	default:
		log.Printf("Job queue is full, job %s for model %s dropped", job.ID, job.ModelName)
	}
}

// GetUpdatesChannel returns the channel for progress updates
func (w *Worker) GetUpdatesChannel() <-chan ProgressUpdate {
	return w.updates
}

// processJobs is the main worker loop
func (w *Worker) processJobs(workerID int) {
	defer w.wg.Done()
	log.Printf("Worker %d started", workerID)

	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				log.Printf("Worker %d: job queue closed, exiting", workerID)
				return
			}
			log.Printf("Worker %d: processing job %s for model %s", workerID, job.ID, job.ModelName)
			w.processDownload(job)

		case <-w.stopCh:
			log.Printf("Worker %d: received stop signal, exiting", workerID)
			return
		}
	}
}

// findOllamaContainer finds which Ollama container is running
func (w *Worker) findOllamaContainer() string {
	// Check for containers with the Ollama service label
	cmd := exec.Command("docker", "ps", "--filter", "label=com.docker.compose.service=ollama", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Get the list of containers
	containers := strings.TrimSpace(string(output))
	if containers == "" {
		return ""
	}

	// Split into individual container names
	containerList := strings.Split(containers, "\n")

	// If multiple containers, log a warning and use the first one
	if len(containerList) > 1 {
		log.Printf("WARNING: Multiple Ollama containers found (%d) when trying to download model, using the first one: %s",
			len(containerList), containerList[0])
		log.Printf("All Ollama containers: %v", containerList)
		// In the future, this should probably return an error
	}

	return containerList[0]
}

// processDownload handles the actual model download
func (w *Worker) processDownload(job DownloadJob) {
	// Find the Ollama container dynamically
	containerName := w.findOllamaContainer()
	if containerName == "" {
		w.handleError(job, "No Ollama container is running")
		return
	}
	log.Printf("Using Ollama container: %s", containerName)

	// Update job status to processing
	err := UpdateJobStatus(w.db, job.ID, "processing")
	if err != nil {
		log.Printf("Failed to update job status: %v", err)
	}

	// Update model status to downloading
	err = UpdateModelStatus(w.db, job.ModelName, StatusDownloading, 0)
	if err != nil {
		log.Printf("Failed to update model status: %v", err)
	}

	// Send initial update
	w.sendUpdate(ProgressUpdate{
		ModelName: job.ModelName,
		Status:    StatusDownloading,
		Progress:  0,
	})

	// Execute the ollama pull command
	cmd := exec.Command("docker", "exec", containerName, "ollama", "pull", job.ModelName)

	// Create pipe for stderr (ollama outputs to stderr)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		w.handleError(job, fmt.Sprintf("Failed to create stderr pipe: %v", err))
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		w.handleError(job, fmt.Sprintf("Failed to start ollama pull: %v", err))
		return
	}

	// Read and parse output from stderr - using a reader to handle carriage returns
	reader := bufio.NewReader(stderr)
	var lastProgress int
	var buffer []byte

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error reading output: %v", err)
			}
			break
		}

		// Handle carriage return or newline - process the line
		if b == '\r' || b == '\n' {
			if len(buffer) > 0 {
				line := string(buffer)

				// Parse progress from the output
				progress := ParseProgress(line)
				if progress > 0 {
					log.Printf("Parsed progress: %d%%", progress)
					if progress != lastProgress {
						lastProgress = progress

						// Update database
						err = UpdateModelStatus(w.db, job.ModelName, StatusDownloading, progress)
						if err != nil {
							log.Printf("Failed to update progress: %v", err)
						}

						// Send progress update
						w.sendUpdate(ProgressUpdate{
							ModelName: job.ModelName,
							Status:    StatusDownloading,
							Progress:  progress,
						})
					}
				} else {
					// Log raw line for debugging non-progress lines
					cleanedLine := strings.TrimSpace(line)
					if cleanedLine != "" && !strings.Contains(cleanedLine, "[K") {
						log.Printf("Ollama output: %s", cleanedLine)
					}
				}
				buffer = buffer[:0]
			}
		} else {
			buffer = append(buffer, b)
		}
	}

	// Wait for command to complete
	err = cmd.Wait()
	if err != nil {
		w.handleError(job, fmt.Sprintf("Ollama pull failed: %v", err))
		return
	}

	// Mark as completed
	err = UpdateModelStatus(w.db, job.ModelName, StatusCompleted, 100)
	if err != nil {
		log.Printf("Failed to update model completion: %v", err)
	}

	err = UpdateJobStatus(w.db, job.ID, "completed")
	if err != nil {
		log.Printf("Failed to update job completion: %v", err)
	}

	// Send completion update
	w.sendUpdate(ProgressUpdate{
		ModelName: job.ModelName,
		Status:    StatusCompleted,
		Progress:  100,
	})

	log.Printf("Successfully downloaded model %s", job.ModelName)
}

// handleError handles download errors
func (w *Worker) handleError(job DownloadJob, errorMsg string) {
	log.Printf("Error downloading model %s: %s", job.ModelName, errorMsg)

	// Update model error state
	err := UpdateModelError(w.db, job.ModelName, errorMsg)
	if err != nil {
		log.Printf("Failed to update model error: %v", err)
	}

	// Update job status
	err = UpdateJobStatus(w.db, job.ID, "failed")
	if err != nil {
		log.Printf("Failed to update job failure: %v", err)
	}

	// Send error update
	w.sendUpdate(ProgressUpdate{
		ModelName: job.ModelName,
		Status:    StatusFailed,
		Progress:  0,
		Error:     errorMsg,
	})
}

// sendUpdate sends a progress update through the updates channel
func (w *Worker) sendUpdate(update ProgressUpdate) {
	select {
	case w.updates <- update:
		// Update sent successfully
	case <-time.After(100 * time.Millisecond):
		// Update channel is blocked, skip this update
		log.Printf("Failed to send update for model %s (channel blocked)", update.ModelName)
	}
}

// recoverPendingJobs recovers any pending jobs on startup
func (w *Worker) recoverPendingJobs() {
	jobs, err := GetPendingJobs(w.db)
	if err != nil {
		log.Printf("Failed to recover pending jobs: %v", err)
		return
	}

	for _, job := range jobs {
		log.Printf("Recovering pending job %s for model %s", job.ID, job.ModelName)
		w.AddJob(job)
	}
}

// startCleanupTask starts a periodic cleanup of old jobs
func (w *Worker) startCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := CleanupOldJobs(w.db, 7*24*time.Hour) // Clean jobs older than 7 days
			if err != nil {
				log.Printf("Failed to cleanup old jobs: %v", err)
			}
		case <-w.stopCh:
			return
		}
	}
}

// ParseProgress extracts progress percentage from ollama output
func ParseProgress(line string) int {
	// First, handle the ANSI escape codes that ollama uses
	// Remove all ANSI escape sequences

	// Remove the [?2026h, [?2026l, [?25h, [?25l sequences
	line = strings.ReplaceAll(line, "[?2026h", "")
	line = strings.ReplaceAll(line, "[?2026l", "")
	line = strings.ReplaceAll(line, "[?25h", "")
	line = strings.ReplaceAll(line, "[?25l", "")

	// Remove cursor position sequences like [1G, [K, [A
	line = strings.ReplaceAll(line, "[1G", "")
	line = strings.ReplaceAll(line, "[K", "")
	line = strings.ReplaceAll(line, "[A", "")

	// Remove carriage returns and newlines
	line = strings.ReplaceAll(line, "\r", "")
	line = strings.ReplaceAll(line, "\n", "")

	// Trim spaces
	line = strings.TrimSpace(line)

	// Log cleaned line for debugging
	if line != "" && !strings.Contains(line, "⠋") && !strings.Contains(line, "⠙") &&
		!strings.Contains(line, "⠹") && !strings.Contains(line, "⠸") &&
		!strings.Contains(line, "⠼") && !strings.Contains(line, "⠴") &&
		!strings.Contains(line, "⠦") && !strings.Contains(line, "⠧") &&
		!strings.Contains(line, "⠇") && !strings.Contains(line, "⠏") {
		log.Printf("ParseProgress: cleaned line = '%s'", line)
	}

	// Check for percentage in the format "pulling 74701a8c35f6... 100%"
	if strings.Contains(line, "%") {
		// Find the percentage value
		parts := strings.Fields(line)
		for _, part := range parts {
			if strings.HasSuffix(part, "%") {
				// Remove % and parse
				percentStr := strings.TrimSuffix(part, "%")
				var percent int
				_, err := fmt.Sscanf(percentStr, "%d", &percent)
				if err == nil && percent >= 0 && percent <= 100 {
					return percent
				}
			}
		}
	}

	// Map specific messages to progress values
	switch {
	case strings.Contains(line, "pulling manifest"):
		return 5
	case strings.Contains(line, "verifying sha256 digest"):
		return 95
	case strings.Contains(line, "writing manifest"):
		return 98
	case strings.Contains(line, "success"):
		return 100
	case strings.Contains(line, "pulling") && strings.Contains(line, "...") && !strings.Contains(line, "%"):
		// Initial pulling without percentage
		return 10
	}

	return 0
}
