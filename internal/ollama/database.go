// Package ollama provides utilities for managing Ollama models and downloads in TreeOS.
package ollama

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
	"treeos/internal/logging"

	"github.com/google/uuid"
)

// InitializeModels is now a no-op kept for backward compatibility.
// Curated models are presented from in-memory metadata and persisted
// only once they are actively downloaded by the user.
func InitializeModels(_ *sql.DB) error {
	return nil
}

// GetAllModels retrieves all models from the database
func GetAllModels(db *sql.DB) ([]OllamaModel, error) {
	records, err := fetchAllModelRecords(db)
	if err != nil {
		return nil, err
	}

	curatedState := make(map[string]OllamaModel)
	var customModels []OllamaModel

	for _, record := range records {
		if _, ok := GetCuratedModel(record.Name); ok {
			curatedState[record.Name] = record
			continue
		}
		customModels = append(customModels, record)
	}

	result := make([]OllamaModel, 0, len(CuratedModels)+len(customModels))
	for _, curated := range CuratedModels {
		merged := curated
		if record, ok := curatedState[curated.Name]; ok {
			merged.Status = record.Status
			merged.Progress = record.Progress
			merged.LastError = record.LastError
			merged.UpdatedAt = record.UpdatedAt
			merged.CompletedAt = record.CompletedAt
		} else {
			merged.Status = StatusNotDownloaded
			merged.Progress = 0
			merged.LastError = sql.NullString{}
			merged.UpdatedAt = time.Time{}
			merged.CompletedAt = sql.NullTime{}
		}
		result = append(result, merged)
	}

	sort.Slice(customModels, func(i, j int) bool {
		left := strings.ToLower(customModels[i].DisplayName)
		right := strings.ToLower(customModels[j].DisplayName)
		if left == right {
			return customModels[i].Name < customModels[j].Name
		}
		return left < right
	})

	result = append(result, customModels...)
	return result, nil
}

func fetchAllModelRecords(db *sql.DB) ([]OllamaModel, error) {
	rows, err := db.Query(`
		SELECT name, display_name, size_estimate, description, category,
		       status, progress, last_error, updated_at, completed_at
		FROM ollama_models`)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	defer rows.Close() //nolint:errcheck // Cleanup, error not critical

	var models []OllamaModel
	for rows.Next() {
		var m OllamaModel
		err := rows.Scan(&m.Name, &m.DisplayName, &m.SizeEstimate, &m.Description,
			&m.Category, &m.Status, &m.Progress, &m.LastError, &m.UpdatedAt, &m.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}
		models = append(models, m)
	}
	return models, nil
}

// GetModel retrieves a single model by name
func GetModel(db *sql.DB, name string) (*OllamaModel, error) {
	var m OllamaModel
	err := db.QueryRow(`
		SELECT name, display_name, size_estimate, description, category,
		       status, progress, last_error, updated_at, completed_at
		FROM ollama_models WHERE name = ?`, name).Scan(
		&m.Name, &m.DisplayName, &m.SizeEstimate, &m.Description,
		&m.Category, &m.Status, &m.Progress, &m.LastError, &m.UpdatedAt, &m.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	return &m, nil
}

// UpdateModelStatus updates a model's status and progress
func UpdateModelStatus(db *sql.DB, name string, status string, progress int) error {
	query := `UPDATE ollama_models 
		SET status = ?, progress = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE name = ?`
	args := []any{status, progress, name}

	switch status {
	case StatusCompleted:
		query = `UPDATE ollama_models 
			SET status = ?, progress = ?, last_error = NULL, updated_at = CURRENT_TIMESTAMP, 
			    completed_at = CURRENT_TIMESTAMP 
			WHERE name = ?`
	case StatusNotDownloaded:
		query = `UPDATE ollama_models 
			SET status = ?, progress = ?, last_error = NULL, updated_at = CURRENT_TIMESTAMP, 
			    completed_at = NULL 
			WHERE name = ?`
	}

	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update model status: %w", err)
	}
	return nil
}

// UpdateModelError updates a model's error state
func UpdateModelError(db *sql.DB, name string, errorMsg string) error {
	_, err := db.Exec(`
		UPDATE ollama_models 
		SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE name = ?`,
		StatusFailed, errorMsg, name)
	if err != nil {
		return fmt.Errorf("failed to update model error: %w", err)
	}
	return nil
}

// ClearModelError clears a model's error state for retry
func ClearModelError(db *sql.DB, name string) error {
	_, err := db.Exec(`
		UPDATE ollama_models
		SET last_error = NULL, status = ?, progress = 0, updated_at = CURRENT_TIMESTAMP
		WHERE name = ?`,
		StatusQueued, name)
	if err != nil {
		return fmt.Errorf("failed to clear model error: %w", err)
	}
	return nil
}

// CreateModel inserts a new model into the database (used for custom models)
func CreateModel(db *sql.DB, model *OllamaModel) error {
	_, err := db.Exec(`
		INSERT INTO ollama_models (name, display_name, category, description, size_estimate, status, progress, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			display_name = excluded.display_name,
			category = excluded.category,
			description = excluded.description,
			status = excluded.status,
			progress = excluded.progress,
			last_error = NULL,
			updated_at = CURRENT_TIMESTAMP`,
		model.Name, model.DisplayName, model.Category, model.Description, model.SizeEstimate, model.Status, model.Progress)
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}
	return nil
}

// CreateDownloadJob creates a new download job
func CreateDownloadJob(db *sql.DB, modelName string) (*DownloadJob, error) {
	if err := ensureModelRecord(db, modelName); err != nil {
		return nil, err
	}

	job := &DownloadJob{
		ID:        uuid.New().String(),
		ModelName: modelName,
		Status:    "queued",
		CreatedAt: time.Now(),
	}

	_, err := db.Exec(`
		INSERT INTO ollama_download_jobs (id, model_name, status, created_at)
		VALUES (?, ?, ?, ?)`,
		job.ID, job.ModelName, job.Status, job.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create download job: %w", err)
	}

	// Also update the model status to queued
	err = UpdateModelStatus(db, modelName, StatusQueued, 0)
	if err != nil {
		logging.Errorf("Failed to update model status to queued: %v", err)
	}

	return job, nil
}

func ensureModelRecord(db *sql.DB, modelName string) error {
	model, err := GetModel(db, modelName)
	if err != nil {
		return err
	}
	if model != nil {
		return nil
	}

	if curated, ok := GetCuratedModel(modelName); ok {
		curatedCopy := *curated
		curatedCopy.Status = StatusNotDownloaded
		curatedCopy.Progress = 0
		curatedCopy.LastError = sql.NullString{}
		curatedCopy.CompletedAt = sql.NullTime{}
		return CreateModel(db, &curatedCopy)
	}

	return fmt.Errorf("model %s is not registered", modelName)
}

// UpdateJobStatus updates a download job's status
func UpdateJobStatus(db *sql.DB, jobID string, status string) error {
	query := `UPDATE ollama_download_jobs SET status = ? WHERE id = ?`
	if status == "processing" {
		query = `UPDATE ollama_download_jobs 
			SET status = ?, started_at = CURRENT_TIMESTAMP 
			WHERE id = ?`
	}

	_, err := db.Exec(query, status, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	return nil
}

// GetPendingJobs retrieves all pending download jobs
func GetPendingJobs(db *sql.DB) ([]DownloadJob, error) {
	rows, err := db.Query(`
		SELECT id, model_name, status, created_at, started_at
		FROM ollama_download_jobs
		WHERE status IN ('queued', 'processing')
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending jobs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // Cleanup, error not critical

	var jobs []DownloadJob
	for rows.Next() {
		var j DownloadJob
		err := rows.Scan(&j.ID, &j.ModelName, &j.Status, &j.CreatedAt, &j.StartedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// CleanupOldJobs removes completed or failed jobs older than specified duration
func CleanupOldJobs(db *sql.DB, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.Exec(`
		DELETE FROM ollama_download_jobs
		WHERE status IN ('completed', 'failed')
		AND created_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old jobs: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if count > 0 {
		logging.Infof("Cleaned up %d old download jobs", count)
	}
	return nil
}

// GetCompletedModels retrieves all models that have been successfully downloaded
func GetCompletedModels(db *sql.DB) ([]OllamaModel, error) {
	rows, err := db.Query(`
		SELECT name, display_name, size_estimate, description, category, 
		       status, progress, last_error, updated_at, completed_at
		FROM ollama_models
		WHERE status = ?
		ORDER BY display_name`, StatusCompleted)
	if err != nil {
		return nil, fmt.Errorf("failed to query completed models: %w", err)
	}
	defer rows.Close() //nolint:errcheck // Cleanup, error not critical

	var models []OllamaModel
	for rows.Next() {
		var m OllamaModel
		err := rows.Scan(&m.Name, &m.DisplayName, &m.SizeEstimate, &m.Description,
			&m.Category, &m.Status, &m.Progress, &m.LastError, &m.UpdatedAt, &m.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}
		models = append(models, m)
	}
	return models, nil
}
