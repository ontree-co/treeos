package server

import (
	"sync"
	"time"
)

// UpdateStatus tracks the current update operation status
type UpdateStatus struct {
	mu         sync.RWMutex
	InProgress bool      `json:"in_progress"`
	Success    bool      `json:"success"`
	Failed     bool      `json:"failed"`
	Error      string    `json:"error,omitempty"`
	Message    string    `json:"message,omitempty"`
	Stage      string    `json:"stage,omitempty"`
	Percentage float64   `json:"percentage"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Global update status (simple solution for now)
var currentUpdateStatus = &UpdateStatus{}

// GetUpdateStatus returns the current update status
func GetUpdateStatus() UpdateStatus {
	currentUpdateStatus.mu.RLock()
	defer currentUpdateStatus.mu.RUnlock()

	// Return a copy without the mutex
	return UpdateStatus{
		InProgress: currentUpdateStatus.InProgress,
		Success:    currentUpdateStatus.Success,
		Failed:     currentUpdateStatus.Failed,
		Error:      currentUpdateStatus.Error,
		Message:    currentUpdateStatus.Message,
		Stage:      currentUpdateStatus.Stage,
		Percentage: currentUpdateStatus.Percentage,
		StartedAt:  currentUpdateStatus.StartedAt,
		UpdatedAt:  currentUpdateStatus.UpdatedAt,
	}
}

// SetUpdateStatus updates the current status
func SetUpdateStatus(status UpdateStatus) {
	currentUpdateStatus.mu.Lock()
	defer currentUpdateStatus.mu.Unlock()

	// Only update the data fields, not the mutex
	currentUpdateStatus.InProgress = status.InProgress
	currentUpdateStatus.Success = status.Success
	currentUpdateStatus.Failed = status.Failed
	currentUpdateStatus.Error = status.Error
	currentUpdateStatus.Message = status.Message
	currentUpdateStatus.Stage = status.Stage
	currentUpdateStatus.Percentage = status.Percentage
	currentUpdateStatus.StartedAt = status.StartedAt
	currentUpdateStatus.UpdatedAt = time.Now()
}

// ResetUpdateStatus clears the update status
func ResetUpdateStatus() {
	currentUpdateStatus.mu.Lock()
	defer currentUpdateStatus.mu.Unlock()
	*currentUpdateStatus = UpdateStatus{
		UpdatedAt: time.Now(),
	}
}