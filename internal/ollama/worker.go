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
	// Track active downloads for cancellation
	activeMu      sync.Mutex
	activeDownloads map[string]*exec.Cmd
}

// NewWorker creates a new worker instance
func NewWorker(db *sql.DB, containerName string) *Worker {
	// Use a default if none provided
	if containerName == "" {
		containerName = "ontree-ollama-ollama-1" // Fallback name
	}

	return &Worker{
		db:            db,
		jobQueue:      make(chan DownloadJob, 100),
		updates:       make(chan ProgressUpdate, 1000),
		stopCh:        make(chan struct{}),
		containerName: containerName,
		activeDownloads: make(map[string]*exec.Cmd),
	}
}

// SetContainerName updates the container name (useful if container is recreated)
func (w *Worker) SetContainerName(containerName string) {
	if containerName != "" {
		w.containerName = containerName
		log.Printf("Updated Ollama worker container name to: %s", containerName)
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

// CancelDownload cancels an active download for a specific model
func (w *Worker) CancelDownload(modelName string) error {
	w.activeMu.Lock()
	defer w.activeMu.Unlock()

	cmd, exists := w.activeDownloads[modelName]
	if !exists {
		return fmt.Errorf("no active download found for model %s", modelName)
	}

	// Kill the process
	if cmd.Process != nil {
		log.Printf("Cancelling download for model %s (PID: %d)", modelName, cmd.Process.Pid)
		err := cmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("failed to kill download process: %v", err)
		}

		// Wait a moment for the process to actually die
		go func() {
			cmd.Wait() // This will clean up the zombie process
		}()
		time.Sleep(500 * time.Millisecond) // Give it a moment to clean up
	}

	// Remove from active downloads
	delete(w.activeDownloads, modelName)

	// Try to clean up partial download immediately
	// Note: This cleanup is also done in the handler, but we do it here too for redundancy
	cleanupCmd := exec.Command("docker", "exec", w.containerName, "ollama", "rm", modelName)
	cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput()
	if cleanupErr == nil {
		log.Printf("Worker cleaned up partial download for model %s", modelName)
	} else if !strings.Contains(string(cleanupOutput), "not found") {
		log.Printf("Worker could not clean up partial model %s: %v", modelName, cleanupErr)
	}

	// Send cancellation update
	w.sendUpdate(ProgressUpdate{
		ModelName: modelName,
		Status:    StatusNotDownloaded,
		Progress:  0,
		Error:     "Download cancelled by user",
	})

	return nil
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

// processDownload handles the actual model download
func (w *Worker) processDownload(job DownloadJob) {
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
	cmd := exec.Command("docker", "exec", w.containerName, "ollama", "pull", job.ModelName)

	// Track this command for potential cancellation
	w.activeMu.Lock()
	w.activeDownloads[job.ModelName] = cmd
	w.activeMu.Unlock()

	// Ensure we clean up when done
	defer func() {
		w.activeMu.Lock()
		delete(w.activeDownloads, job.ModelName)
		w.activeMu.Unlock()
	}()

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
		// Check if this was a cancellation (process killed)
		w.activeMu.Lock()
		_, stillActive := w.activeDownloads[job.ModelName]
		w.activeMu.Unlock()

		if !stillActive {
			// This was cancelled, don't treat as error
			log.Printf("Download cancelled for model %s", job.ModelName)
			return
		}

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
	log.Printf("Sending update for model %s: status=%s, progress=%d%%",
		update.ModelName, update.Status, update.Progress)

	select {
	case w.updates <- update:
		// Update sent successfully
		log.Printf("Update sent successfully for model %s", update.ModelName)
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
