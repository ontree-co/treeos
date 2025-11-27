package ollama

import (
	"bufio"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"treeos/internal/logging"
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
	activeMu        sync.Mutex
	activeDownloads map[string]*exec.Cmd
}

// NewWorker creates a new worker instance
func NewWorker(db *sql.DB, containerName string) *Worker {
	// Use a default if none provided
	if containerName == "" {
		containerName = "ontree-ollama-ollama-1" // Fallback name
	}

	return &Worker{
		db:              db,
		jobQueue:        make(chan DownloadJob, 100),
		updates:         make(chan ProgressUpdate, 1000),
		stopCh:          make(chan struct{}),
		containerName:   containerName,
		activeDownloads: make(map[string]*exec.Cmd),
	}
}

// SetContainerName updates the container name (useful if container is recreated)
func (w *Worker) SetContainerName(containerName string) {
	if containerName != "" {
		w.containerName = containerName
		logging.Infof("Updated Ollama worker container name to: %s", containerName)
	}
}

// Start initializes and starts the worker pool
func (w *Worker) Start(numWorkers int) {
	logging.Infof("Starting Ollama worker pool with %d workers", numWorkers)

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
	logging.Info("Stopping Ollama worker pool")
	close(w.stopCh)
	w.wg.Wait()
	close(w.jobQueue)
	close(w.updates)
}

// AddJob adds a new download job to the queue
func (w *Worker) AddJob(job DownloadJob) {
	select {
	case w.jobQueue <- job:
		logging.Infof("Added job %s for model %s to queue", job.ID, job.ModelName)
	default:
		logging.Infof("Job queue is full, job %s for model %s dropped", job.ID, job.ModelName)
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

	// First, try to kill any ollama pull processes inside the container
	// This is necessary because killing the container exec process doesn't propagate to the container
	// We need to discover the container name in case it changed
	containerName := w.containerName
	if containerName == "" {
		// Try to discover it
		if discovered, err := w.discoverOllamaContainer(); err == nil {
			containerName = discovered
		}
	}

	if containerName != "" {
		//nolint:gosec // Container name validated from discovery, model name from request
		killInsideCmd := exec.Command("docker", "exec", containerName, "sh", "-c",
			fmt.Sprintf("pkill -f 'ollama pull %s' || true", modelName))
		if err := killInsideCmd.Run(); err != nil {
			logging.Warnf("Warning: Failed to kill ollama process inside container: %v", err)
		}
	} else {
		logging.Warnf("Warning: No container name available to kill ollama process inside container")
	}

	// Then kill the container exec process
	if cmd.Process != nil {
		logging.Infof("Cancelling download for model %s (PID: %d)", modelName, cmd.Process.Pid)
		err := cmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("failed to kill download process: %v", err)
		}

		// Wait a moment for the process to actually die
		go func() {
			_ = cmd.Wait() // This will clean up the zombie process
		}()
		time.Sleep(500 * time.Millisecond) // Give it a moment to clean up
	}

	// Remove from active downloads
	delete(w.activeDownloads, modelName)

	// Try to clean up partial download immediately
	// Note: This cleanup is also done in the handler, but we do it here too for redundancy
	cleanupCmd := exec.Command("docker", "exec", w.containerName, "ollama", "rm", modelName) //nolint:gosec // containerName and modelName are validated
	cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput()
	if cleanupErr == nil {
		logging.Infof("Worker cleaned up partial download for model %s", modelName)
	} else if !strings.Contains(string(cleanupOutput), "not found") {
		logging.Infof("Worker could not clean up partial model %s: %v", modelName, cleanupErr)
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
	logging.Infof("Worker %d started", workerID)

	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				logging.Infof("Worker %d: job queue closed, exiting", workerID)
				return
			}
			logging.Infof("Worker %d: processing job %s for model %s", workerID, job.ID, job.ModelName)
			w.processDownload(job)

		case <-w.stopCh:
			logging.Infof("Worker %d: received stop signal, exiting", workerID)
			return
		}
	}
}

// discoverOllamaContainer finds the running Ollama container using label-based detection
func (w *Worker) discoverOllamaContainer() (string, error) {
	// Look for containers with the ontree.inference=true label
	cmd := exec.Command("docker", "ps", "--filter", "label=ontree.inference=true", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	// Parse output
	containers := strings.TrimSpace(string(output))
	if containers == "" {
		return "", fmt.Errorf("no Ollama container is running")
	}

	// Split into individual container names
	containerList := strings.Split(containers, "\n")

	// If multiple containers found, return error
	if len(containerList) > 1 {
		return "", fmt.Errorf("multiple Ollama containers found (%d containers). Please ensure only one is running", len(containerList))
	}

	containerName := strings.TrimSpace(containerList[0])
	logging.Infof("Discovered Ollama container: %s", containerName)
	return containerName, nil
}

// processDownload handles the actual model download
func (w *Worker) processDownload(job DownloadJob) {
	// Update job status to processing
	err := UpdateJobStatus(w.db, job.ID, "processing")
	if err != nil {
		logging.Errorf("Failed to update job status: %v", err)
	}

	// Discover the Ollama container dynamically
	containerName, err := w.discoverOllamaContainer()
	if err != nil {
		w.handleError(job, fmt.Sprintf("Failed to discover Ollama container: %v", err))
		return
	}

	// Update model status to downloading
	err = UpdateModelStatus(w.db, job.ModelName, StatusDownloading, 0)
	if err != nil {
		logging.Errorf("Failed to update model status: %v", err)
	}

	// Send initial update
	w.sendUpdate(ProgressUpdate{
		ModelName: job.ModelName,
		Status:    StatusDownloading,
		Progress:  0,
	})

	// Execute the ollama pull command
	//nolint:gosec // Container name validated from discovery, model name from request
	cmd := exec.Command("docker", "exec", containerName, "ollama", "pull", job.ModelName)

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
				logging.Errorf("Error reading output: %v", err)
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
					logging.Infof("Parsed progress: %d%%", progress)
					if progress != lastProgress {
						lastProgress = progress

						// Update database
						err = UpdateModelStatus(w.db, job.ModelName, StatusDownloading, progress)
						if err != nil {
							logging.Errorf("Failed to update progress: %v", err)
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
						logging.Infof("Ollama output: %s", cleanedLine)
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
			logging.Infof("Download cancelled for model %s", job.ModelName)
			return
		}

		w.handleError(job, fmt.Sprintf("Ollama pull failed: %v", err))
		return
	}

	// Mark as completed
	err = UpdateModelStatus(w.db, job.ModelName, StatusCompleted, 100)
	if err != nil {
		logging.Errorf("Failed to update model completion: %v", err)
	}

	err = UpdateJobStatus(w.db, job.ID, "completed")
	if err != nil {
		logging.Errorf("Failed to update job completion: %v", err)
	}

	// Send completion update
	w.sendUpdate(ProgressUpdate{
		ModelName: job.ModelName,
		Status:    StatusCompleted,
		Progress:  100,
	})

	logging.Infof("Successfully downloaded model %s", job.ModelName)
}

// handleError handles download errors
func (w *Worker) handleError(job DownloadJob, errorMsg string) {
	logging.Errorf("Error downloading model %s: %s", job.ModelName, errorMsg)

	// Update model error state
	err := UpdateModelError(w.db, job.ModelName, errorMsg)
	if err != nil {
		logging.Errorf("Failed to update model error: %v", err)
	}

	// Update job status
	err = UpdateJobStatus(w.db, job.ID, "failed")
	if err != nil {
		logging.Errorf("Failed to update job failure: %v", err)
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
	logging.Infof("Sending update for model %s: status=%s, progress=%d%%",
		update.ModelName, update.Status, update.Progress)

	select {
	case w.updates <- update:
		// Update sent successfully
		logging.Infof("Update sent successfully for model %s", update.ModelName)
	case <-time.After(100 * time.Millisecond):
		// Update channel is blocked, skip this update
		logging.Errorf("Failed to send update for model %s (channel blocked)", update.ModelName)
	}
}

// recoverPendingJobs recovers any pending jobs on startup
func (w *Worker) recoverPendingJobs() {
	jobs, err := GetPendingJobs(w.db)
	if err != nil {
		logging.Errorf("Failed to recover pending jobs: %v", err)
		return
	}

	for _, job := range jobs {
		logging.Infof("Recovering pending job %s for model %s", job.ID, job.ModelName)
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
				logging.Errorf("Failed to cleanup old jobs: %v", err)
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
		logging.Infof("ParseProgress: cleaned line = '%s'", line)
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
