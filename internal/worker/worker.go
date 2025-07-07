package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"ontree-node/internal/database"
	"ontree-node/internal/docker"
)

// Worker manages background processing of Docker operations
type Worker struct {
	db         *sql.DB
	dockerSvc  *docker.Service
	jobQueue   chan string
	workerWg   sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// New creates a new Worker instance for processing Docker operations
func New(db *sql.DB, dockerSvc *docker.Service) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		db:         db,
		dockerSvc:  dockerSvc,
		jobQueue:   make(chan string, 100),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// Start begins processing operations with the specified number of workers
func (w *Worker) Start(numWorkers int) {
	// Start cleanup goroutine for stale operations
	w.workerWg.Add(1)
	go w.cleanupStaleOperations()

	for i := 0; i < numWorkers; i++ {
		w.workerWg.Add(1)
		go w.worker(i)
	}

	// Pick up any pending operations on startup
	go w.pickupPendingOperations()
}

// pickupPendingOperations finds and enqueues any pending operations from the database
func (w *Worker) pickupPendingOperations() {
	// Wait a moment for workers to be ready
	time.Sleep(1 * time.Second)

	query := `
		SELECT id FROM docker_operations 
		WHERE status = ? 
		ORDER BY created_at ASC
	`

	rows, err := w.db.Query(query, database.StatusPending)
	if err != nil {
		log.Printf("Failed to query pending operations: %v", err)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	count := 0
	for rows.Next() {
		var operationID string
		if err := rows.Scan(&operationID); err != nil {
			log.Printf("Failed to scan operation ID: %v", err)
			continue
		}

		w.EnqueueOperation(operationID)
		count++
	}

	if count > 0 {
		log.Printf("Picked up %d pending operations on startup", count)
	}
}

// Stop gracefully shuts down all workers
func (w *Worker) Stop() {
	w.cancelFunc()
	close(w.jobQueue)
	w.workerWg.Wait()
}

// EnqueueOperation adds an operation to the processing queue
func (w *Worker) EnqueueOperation(operationID string) {
	select {
	case w.jobQueue <- operationID:
		log.Printf("Enqueued operation: %s", operationID)
	default:
		log.Printf("Job queue full, dropping operation: %s", operationID)
	}
}

func (w *Worker) worker(id int) {
	defer w.workerWg.Done()
	log.Printf("Worker %d started", id)

	for {
		select {
		case operationID, ok := <-w.jobQueue:
			if !ok {
				log.Printf("Worker %d stopping", id)
				return
			}
			w.processOperation(operationID)
		case <-w.ctx.Done():
			log.Printf("Worker %d stopping due to context cancellation", id)
			return
		}
	}
}

func (w *Worker) processOperation(operationID string) {
	log.Printf("Processing operation: %s", operationID)

	// Create logger for this operation
	logger := NewOperationLogger(w.db, operationID)

	// Get operation details from database
	var op database.DockerOperation
	var metadataStr string
	err := w.db.QueryRow(
		"SELECT id, operation_type, app_name, status, metadata FROM docker_operations WHERE id = ?",
		operationID).Scan(&op.ID, &op.OperationType, &op.AppName, &op.Status, &metadataStr)
	if err == nil {
		op.Metadata = json.RawMessage(metadataStr)
	}
	if err != nil {
		log.Printf("Failed to get operation %s: %v", operationID, err)
		return
	}

	// Log operation start
	logger.LogInfo(fmt.Sprintf("Operation started: %s for app '%s'", op.OperationType, op.AppName))

	// Skip if not pending
	if op.Status != database.StatusPending {
		logger.LogWarning(fmt.Sprintf("Operation %s is not pending (status: %s), skipping", operationID, op.Status))
		log.Printf("Operation %s is not pending (status: %s), skipping", operationID, op.Status)
		return
	}

	// Log worker assignment
	logger.LogInfo(fmt.Sprintf("Worker picked up operation %s", operationID))

	// Update status to in_progress
	w.updateOperationStatus(operationID, database.StatusInProgress, 0, "Starting operation", "")

	// Execute the operation based on type
	var operationErr error
	switch op.OperationType {
	case database.OpTypeStartContainer:
		operationErr = w.processStartOperation(&op, logger)
	case database.OpTypeRecreateContainer:
		operationErr = w.processRecreateOperation(&op, logger)
	case database.OpTypeUpdateImage:
		operationErr = w.processUpdateImageOperation(&op, logger)
	default:
		operationErr = fmt.Errorf("unknown operation type: %s", op.OperationType)
		logger.LogError(fmt.Sprintf("Unknown operation type: %s", op.OperationType))
	}

	// Update final status
	if operationErr != nil {
		logger.LogError(fmt.Sprintf("Operation failed: %v", operationErr))
		w.updateOperationStatus(operationID, database.StatusFailed, 0, "", operationErr.Error())
	} else {
		logger.LogInfo("Operation completed successfully")
		w.updateOperationStatus(operationID, database.StatusCompleted, 100, "Operation completed successfully", "")
	}
}

func (w *Worker) processStartOperation(op *database.DockerOperation, logger *OperationLogger) error {
	log.Printf("Starting app: %s", op.AppName)
	logger.LogInfo("Starting container creation process")

	// Update progress
	w.updateOperationStatus(op.ID, database.StatusInProgress, 10, "Checking container status", "")
	logger.LogInfo("Checking container status...")

	// Get app details
	app, err := w.dockerSvc.GetAppDetails(op.AppName)
	if err != nil {
		logger.LogError(fmt.Sprintf("Failed to get app details: %v", err))
		return fmt.Errorf("failed to get app details: %w", err)
	}

	logger.LogInfo(fmt.Sprintf("App status: %s", app.Status))
	if app.Config != nil && app.Config.Container.Image != "" {
		logger.LogInfo(fmt.Sprintf("Container image: %s", app.Config.Container.Image))
	}

	// Check if we need to pull images
	if app.Status == "not_created" {
		logger.LogInfo("Container not found, will create new container")
		w.updateOperationStatus(op.ID, database.StatusInProgress, 20, "Pulling Docker images", "")

		// Log the pull command equivalent
		if app.Config != nil && app.Config.Container.Image != "" {
			logger.LogCommand("Pulling Docker image", fmt.Sprintf("docker pull %s", app.Config.Container.Image))
		}

		// Create a progress callback
		progressCallback := func(progress int, message string) {
			// Scale progress from 20-80 for image pulling
			scaledProgress := 20 + (progress * 60 / 100)
			w.updateOperationStatus(op.ID, database.StatusInProgress, scaledProgress, message, "")
			logger.LogInfo(message)
		}

		// Pull images with progress
		logger.LogInfo("Starting image pull...")
		if err := w.dockerSvc.PullImagesWithProgress(op.AppName, progressCallback); err != nil {
			logger.LogError(fmt.Sprintf("Failed to pull images: %v", err))
			return fmt.Errorf("failed to pull images: %w", err)
		}
		logger.LogInfo("Image pull completed successfully")
	} else {
		logger.LogInfo(fmt.Sprintf("Container already exists with status: %s", app.Status))
	}

	// Start the container
	w.updateOperationStatus(op.ID, database.StatusInProgress, 90, "Starting container", "")
	logger.LogInfo("Starting container...")
	logger.LogCommand("Starting container", fmt.Sprintf("docker start ontree-%s", op.AppName))

	if err := w.dockerSvc.StartApp(op.AppName); err != nil {
		logger.LogError(fmt.Sprintf("Failed to start container: %v", err))
		return fmt.Errorf("failed to start app: %w", err)
	}

	logger.LogInfo("Container started successfully")
	return nil
}

func (w *Worker) processRecreateOperation(op *database.DockerOperation, logger *OperationLogger) error {
	log.Printf("Recreating app: %s", op.AppName)
	logger.LogInfo("Starting container recreation process")

	// Update progress
	w.updateOperationStatus(op.ID, database.StatusInProgress, 10, "Stopping existing container", "")

	// Stop existing container
	if err := w.dockerSvc.StopApp(op.AppName); err != nil {
		log.Printf("Warning: failed to stop app %s: %v", op.AppName, err)
	}

	// Remove existing container
	w.updateOperationStatus(op.ID, database.StatusInProgress, 20, "Removing container", "")
	if err := w.dockerSvc.DeleteApp(op.AppName); err != nil {
		log.Printf("Warning: failed to delete app %s: %v", op.AppName, err)
	}

	// Pull latest images
	w.updateOperationStatus(op.ID, database.StatusInProgress, 30, "Pulling Docker images", "")

	progressCallback := func(progress int, message string) {
		// Scale progress from 30-90 for image pulling
		scaledProgress := 30 + (progress * 60 / 100)
		w.updateOperationStatus(op.ID, database.StatusInProgress, scaledProgress, message, "")
	}

	if err := w.dockerSvc.PullImagesWithProgress(op.AppName, progressCallback); err != nil {
		return fmt.Errorf("failed to pull images: %w", err)
	}

	// Start new container
	w.updateOperationStatus(op.ID, database.StatusInProgress, 95, "Starting new container", "")
	if err := w.dockerSvc.StartApp(op.AppName); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	return nil
}

func (w *Worker) processUpdateImageOperation(op *database.DockerOperation, logger *OperationLogger) error {
	log.Printf("Updating image for app: %s", op.AppName)
	logger.LogInfo("Starting image update process")

	// Update progress
	w.updateOperationStatus(op.ID, database.StatusInProgress, 10, "Checking for image updates", "")

	// Check if updates are available
	updateStatus, err := w.dockerSvc.CheckImageUpdate(op.AppName)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !updateStatus.UpdateAvailable {
		w.updateOperationStatus(op.ID, database.StatusInProgress, 100, "Image is already up to date", "")
		return nil
	}

	// Get current container status
	app, err := w.dockerSvc.GetAppDetails(op.AppName)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}

	wasRunning := app.Status == "running"

	// Pull the latest image
	w.updateOperationStatus(op.ID, database.StatusInProgress, 20, "Pulling latest image", "")

	progressCallback := func(progress int, message string) {
		// Scale progress from 20-70 for image pulling
		scaledProgress := 20 + (progress * 50 / 100)
		w.updateOperationStatus(op.ID, database.StatusInProgress, scaledProgress, message, "")
	}

	if err := w.dockerSvc.PullImagesWithProgress(op.AppName, progressCallback); err != nil {
		return fmt.Errorf("failed to pull latest image: %w", err)
	}

	// If container exists, we need to recreate it
	if app.Status != "not_created" {
		// Stop existing container
		w.updateOperationStatus(op.ID, database.StatusInProgress, 75, "Stopping current container", "")
		if err := w.dockerSvc.StopApp(op.AppName); err != nil {
			log.Printf("Warning: failed to stop app %s: %v", op.AppName, err)
		}

		// Remove existing container
		w.updateOperationStatus(op.ID, database.StatusInProgress, 80, "Removing old container", "")
		if err := w.dockerSvc.DeleteApp(op.AppName); err != nil {
			log.Printf("Warning: failed to delete app %s: %v", op.AppName, err)
		}

		// Start new container with updated image
		w.updateOperationStatus(op.ID, database.StatusInProgress, 90, "Starting container with updated image", "")
		if err := w.dockerSvc.StartApp(op.AppName); err != nil {
			return fmt.Errorf("failed to start app with updated image: %w", err)
		}
	}

	// Final status
	if wasRunning {
		w.updateOperationStatus(op.ID, database.StatusInProgress, 100, "Container updated and running", "")
	} else {
		w.updateOperationStatus(op.ID, database.StatusInProgress, 100, "Image updated successfully", "")
	}

	return nil
}

func (w *Worker) updateOperationStatus(operationID, status string, progress int, progressMessage, errorMessage string) {
	query := `
		UPDATE docker_operations 
		SET status = ?, progress = ?, progress_message = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	if status == database.StatusCompleted || status == database.StatusFailed {
		query = `
			UPDATE docker_operations 
			SET status = ?, progress = ?, progress_message = ?, error_message = ?, 
			    updated_at = CURRENT_TIMESTAMP, completed_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`
	}

	_, err := w.db.Exec(query, status, progress, progressMessage, errorMessage, operationID)
	if err != nil {
		log.Printf("Failed to update operation status: %v", err)
	}
}

// cleanupStaleOperations runs periodically to mark old pending operations as failed
func (w *Worker) cleanupStaleOperations() {
	defer w.workerWg.Done()
	log.Printf("Stale operation cleanup started")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Mark operations older than 5 minutes as failed
			query := `
				UPDATE docker_operations 
				SET status = ?, 
				    error_message = 'Operation timed out - worker may have been unavailable', 
				    updated_at = CURRENT_TIMESTAMP,
				    completed_at = CURRENT_TIMESTAMP
				WHERE status IN (?, ?)
				AND created_at <= datetime('now', '-5 minutes')
			`

			result, err := w.db.Exec(query, database.StatusFailed, database.StatusPending, database.StatusInProgress)
			if err != nil {
				log.Printf("Failed to cleanup stale operations: %v", err)
				continue
			}

			affected, err := result.RowsAffected()
			if err != nil {
				log.Printf("Failed to get affected rows: %v", err)
			}
			if affected > 0 {
				log.Printf("Marked %d stale operations as failed", affected)
			}

		case <-w.ctx.Done():
			log.Printf("Stale operation cleanup stopping")
			return
		}
	}
}
