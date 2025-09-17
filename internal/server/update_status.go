package server

import (
	"sync"
	"time"
)

// UpdateStatus tracks the current update operation status
type UpdateStatus struct {
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

// Global update status with mutex protection
var (
	updateStatusMu     sync.RWMutex
	currentUpdateStatus = UpdateStatus{}
)

// GetUpdateStatus returns the current update status
func GetUpdateStatus() UpdateStatus {
	updateStatusMu.RLock()
	defer updateStatusMu.RUnlock()

	// Return a copy
	return currentUpdateStatus
}

// SetUpdateStatus updates the current status
func SetUpdateStatus(status UpdateStatus) {
	updateStatusMu.Lock()
	defer updateStatusMu.Unlock()

	status.UpdatedAt = time.Now()
	currentUpdateStatus = status
}

// ResetUpdateStatus clears the update status
func ResetUpdateStatus() {
	updateStatusMu.Lock()
	defer updateStatusMu.Unlock()
	currentUpdateStatus = UpdateStatus{
		UpdatedAt: time.Now(),
	}
}