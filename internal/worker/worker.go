package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"ontree-node/internal/database"
	"ontree-node/internal/docker"
)

type Worker struct {
	db         *sql.DB
	dockerSvc  *docker.Service
	jobQueue   chan string
	workerWg   sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
}

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

func (w *Worker) Start(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		w.workerWg.Add(1)
		go w.worker(i)
	}
}

func (w *Worker) Stop() {
	w.cancelFunc()
	close(w.jobQueue)
	w.workerWg.Wait()
}

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

	// Get operation details from database
	var op database.DockerOperation
	err := w.db.QueryRow(
		"SELECT id, operation_type, app_name, status, metadata FROM docker_operations WHERE id = ?",
		operationID).Scan(&op.ID, &op.OperationType, &op.AppName, &op.Status, &op.Metadata)
	if err != nil {
		log.Printf("Failed to get operation %s: %v", operationID, err)
		return
	}

	// Skip if not pending
	if op.Status != database.StatusPending {
		log.Printf("Operation %s is not pending (status: %s), skipping", operationID, op.Status)
		return
	}

	// Update status to in_progress
	w.updateOperationStatus(operationID, database.StatusInProgress, 0, "Starting operation", "")

	// Execute the operation based on type
	var operationErr error
	switch op.OperationType {
	case database.OpTypeStartContainer:
		operationErr = w.processStartOperation(&op)
	case database.OpTypeRecreateContainer:
		operationErr = w.processRecreateOperation(&op)
	default:
		operationErr = fmt.Errorf("unknown operation type: %s", op.OperationType)
	}

	// Update final status
	if operationErr != nil {
		w.updateOperationStatus(operationID, database.StatusFailed, 0, "", operationErr.Error())
	} else {
		w.updateOperationStatus(operationID, database.StatusCompleted, 100, "Operation completed successfully", "")
	}
}

func (w *Worker) processStartOperation(op *database.DockerOperation) error {
	log.Printf("Starting app: %s", op.AppName)

	// Update progress
	w.updateOperationStatus(op.ID, database.StatusInProgress, 10, "Checking container status", "")

	// Get app details
	app, err := w.dockerSvc.GetAppDetails(op.AppName)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}

	// Check if we need to pull images
	if app.Status == "not_created" {
		w.updateOperationStatus(op.ID, database.StatusInProgress, 20, "Pulling Docker images", "")

		// Create a progress callback
		progressCallback := func(progress int, message string) {
			// Scale progress from 20-80 for image pulling
			scaledProgress := 20 + (progress * 60 / 100)
			w.updateOperationStatus(op.ID, database.StatusInProgress, scaledProgress, message, "")
		}

		// Pull images with progress
		if err := w.dockerSvc.PullImagesWithProgress(op.AppName, progressCallback); err != nil {
			return fmt.Errorf("failed to pull images: %w", err)
		}
	}

	// Start the container
	w.updateOperationStatus(op.ID, database.StatusInProgress, 90, "Starting container", "")
	if err := w.dockerSvc.StartApp(op.AppName); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	return nil
}

func (w *Worker) processRecreateOperation(op *database.DockerOperation) error {
	log.Printf("Recreating app: %s", op.AppName)

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
