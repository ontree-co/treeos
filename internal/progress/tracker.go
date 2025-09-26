package progress

import (
	"sync"
	"time"
)

// Operation represents different types of operations that can be tracked
type Operation string

const (
	OperationPreparing   Operation = "preparing"
	OperationDownloading Operation = "downloading"
	OperationExtracting  Operation = "extracting"
	OperationStarting    Operation = "starting"
	OperationComplete    Operation = "complete"
	OperationError       Operation = "error"
)

// ImageProgress represents progress for a single container image
type ImageProgress struct {
	Name        string  `json:"name"`
	Progress    float64 `json:"progress"`    // Percentage 0-100
	Downloaded  int64   `json:"downloaded"` // Bytes downloaded
	Total       int64   `json:"total"`      // Total bytes
	Status      string  `json:"status"`     // downloading, extracting, complete
}

// AppProgress represents the overall progress for an app operation
type AppProgress struct {
	AppName               string                    `json:"app_name"`
	Operation             Operation                 `json:"operation"`
	OverallProgress       float64                   `json:"overall_progress"`       // 0-100
	Message               string                    `json:"message"`                // Human readable status
	Details               string                    `json:"details"`                // Additional details
	Images                map[string]*ImageProgress `json:"images"`                 // Per-image progress
	EstimatedTimeRemaining string                   `json:"estimated_time_remaining,omitempty"`
	StartTime             time.Time                 `json:"start_time"`
	LastUpdate            time.Time                 `json:"last_update"`
	Error                 string                    `json:"error,omitempty"`
}

// Tracker manages progress for multiple app operations
type Tracker struct {
	mu    sync.RWMutex
	apps  map[string]*AppProgress
}

// NewTracker creates a new progress tracker
func NewTracker() *Tracker {
	return &Tracker{
		apps: make(map[string]*AppProgress),
	}
}

// StartOperation begins tracking progress for an app operation
func (t *Tracker) StartOperation(appName string, operation Operation, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.apps[appName] = &AppProgress{
		AppName:         appName,
		Operation:       operation,
		OverallProgress: 0,
		Message:         message,
		Images:          make(map[string]*ImageProgress),
		StartTime:       now,
		LastUpdate:      now,
	}
}

// UpdateOperation updates the progress for an app operation
func (t *Tracker) UpdateOperation(appName string, operation Operation, progress float64, message, details string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	app, exists := t.apps[appName]
	if !exists {
		// Create new app progress entry if it doesn't exist
		now := time.Now()
		app = &AppProgress{
			AppName:         appName,
			Operation:       operation,
			OverallProgress: 0,
			Message:         message,
			Images:          make(map[string]*ImageProgress),
			StartTime:       now,
			LastUpdate:      now,
		}
		t.apps[appName] = app
	}

	// Update the progress
	app.Operation = operation
	app.OverallProgress = progress
	app.Message = message
	app.Details = details
	app.LastUpdate = time.Now()

	// Calculate estimated time remaining based on progress
	if progress > 0 && progress < 100 {
		elapsed := time.Since(app.StartTime)
		if progress > 5 { // Only estimate after some meaningful progress
			totalEstimated := time.Duration(float64(elapsed) * (100.0 / progress))
			remaining := totalEstimated - elapsed
			if remaining > 0 {
				app.EstimatedTimeRemaining = formatDuration(remaining)
			}
		}
	}
}

// UpdateImageProgress updates progress for a specific image
func (t *Tracker) UpdateImageProgress(appName, imageName string, downloaded, total int64, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	app, exists := t.apps[appName]
	if !exists {
		// Create new app progress entry if it doesn't exist
		now := time.Now()
		app = &AppProgress{
			AppName:         appName,
			Operation:       OperationDownloading,
			OverallProgress: 0,
			Message:         "Processing images",
			Images:          make(map[string]*ImageProgress),
			StartTime:       now,
			LastUpdate:      now,
		}
		t.apps[appName] = app
	}

	if app.Images == nil {
		app.Images = make(map[string]*ImageProgress)
	}

	progress := float64(0)
	if total > 0 {
		progress = float64(downloaded) / float64(total) * 100
	}

	app.Images[imageName] = &ImageProgress{
		Name:       imageName,
		Progress:   progress,
		Downloaded: downloaded,
		Total:      total,
		Status:     status,
	}

	// Update overall progress based on all images
	t.calculateOverallImageProgress(app)
	app.LastUpdate = time.Now()
}

// calculateOverallImageProgress calculates overall progress from individual images
func (t *Tracker) calculateOverallImageProgress(app *AppProgress) {
	if len(app.Images) == 0 {
		return
	}

	totalProgress := float64(0)
	for _, img := range app.Images {
		totalProgress += img.Progress
	}

	app.OverallProgress = totalProgress / float64(len(app.Images))
}

// SetError marks an operation as failed
func (t *Tracker) SetError(appName, errorMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if app, exists := t.apps[appName]; exists {
		app.Operation = OperationError
		app.Error = errorMsg
		app.Message = "Operation failed"
		app.LastUpdate = time.Now()
	}
}

// CompleteOperation marks an operation as complete
func (t *Tracker) CompleteOperation(appName, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if app, exists := t.apps[appName]; exists {
		app.Operation = OperationComplete
		app.OverallProgress = 100
		app.Message = message
		app.Details = ""
		app.EstimatedTimeRemaining = ""
		app.LastUpdate = time.Now()
	}
}

// GetProgress returns the current progress for an app
func (t *Tracker) GetProgress(appName string) (*AppProgress, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	app, exists := t.apps[appName]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	copy := *app
	copy.Images = make(map[string]*ImageProgress)
	for k, v := range app.Images {
		imgCopy := *v
		copy.Images[k] = &imgCopy
	}

	return &copy, true
}

// RemoveOperation removes tracking for an app operation
func (t *Tracker) RemoveOperation(appName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.apps, appName)
}

// CleanupOldOperations removes operations older than the specified duration
func (t *Tracker) CleanupOldOperations(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for appName, app := range t.apps {
		if app.LastUpdate.Before(cutoff) {
			delete(t.apps, appName)
		}
	}
}

// ListActiveOperations returns all currently tracked operations
func (t *Tracker) ListActiveOperations() map[string]*AppProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*AppProgress)
	for k, v := range t.apps {
		copy := *v
		copy.Images = make(map[string]*ImageProgress)
		for imgK, imgV := range v.Images {
			imgCopy := *imgV
			copy.Images[imgK] = &imgCopy
		}
		result[k] = &copy
	}
	return result
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}

	minutes := int(d.Minutes())
	if minutes < 60 {
		return formatTime(minutes, "m")
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	if remainingMinutes == 0 {
		return formatTime(hours, "h")
	}

	return formatTime(hours, "h") + " " + formatTime(remainingMinutes, "m")
}

func formatTime(value int, unit string) string {
	return formatInt(value) + unit
}

func formatInt(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	if i < 100 {
		return string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	// For larger numbers, use a simple approach
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}