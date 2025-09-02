package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"ontree-node/internal/database"
)

// SystemStatusResponse represents the response for system status endpoints
type SystemStatusResponse struct {
	Timestamp        time.Time `json:"timestamp"`
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryPercent    float64   `json:"memory_percent"`
	DiskUsagePercent float64   `json:"disk_usage_percent"`
	GPULoad          float64   `json:"gpu_load"`
	UploadRate       uint64    `json:"upload_rate"`   // bytes per second
	DownloadRate     uint64    `json:"download_rate"` // bytes per second
}

// handleAPIStatusLatest handles GET /api/v1/status/latest
func (s *Server) handleAPIStatusLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the latest metric from database
	latest, err := database.GetLatestMetric("")
	if err != nil {
		log.Printf("Failed to get latest metric: %v", err)
		http.Error(w, "Failed to get latest metric", http.StatusInternalServerError)
		return
	}

	// If no data, return empty response
	if latest == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "No data available"})
		return
	}

	// Convert to response format
	response := SystemStatusResponse{
		Timestamp:        latest.Timestamp,
		CPUPercent:       latest.CPUPercent,
		MemoryPercent:    latest.MemoryPercent,
		DiskUsagePercent: latest.DiskUsagePercent,
		GPULoad:          latest.GPULoad,
		UploadRate:       latest.UploadRate,
		DownloadRate:     latest.DownloadRate,
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleAPIStatusHistory handles GET /api/v1/status/history
func (s *Server) handleAPIStatusHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters for time range
	query := r.URL.Query()

	// Default to last 24 hours if no parameters
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	// Check for start_time parameter
	if startStr := query.Get("start_time"); startStr != "" {
		if startUnix, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			startTime = time.Unix(startUnix, 0)
		}
	}

	// Check for end_time parameter
	if endStr := query.Get("end_time"); endStr != "" {
		if endUnix, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			endTime = time.Unix(endUnix, 0)
		}
	}

	// Check for range parameter (shorthand for common ranges)
	if rangeStr := query.Get("range"); rangeStr != "" {
		switch rangeStr {
		case "1h":
			startTime = endTime.Add(-1 * time.Hour)
		case "6h":
			startTime = endTime.Add(-6 * time.Hour)
		case "12h":
			startTime = endTime.Add(-12 * time.Hour)
		case "24h":
			startTime = endTime.Add(-24 * time.Hour)
		case "7d":
			startTime = endTime.Add(-7 * 24 * time.Hour)
		}
	}

	// Get metrics for the time range
	metrics, err := database.GetMetricsForTimeRange(startTime, endTime)
	if err != nil {
		log.Printf("Failed to get metrics for time range: %v", err)
		http.Error(w, "Failed to get metrics", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var response []SystemStatusResponse
	for _, m := range metrics {
		response = append(response, SystemStatusResponse{
			Timestamp:        m.Timestamp,
			CPUPercent:       m.CPUPercent,
			MemoryPercent:    m.MemoryPercent,
			DiskUsagePercent: m.DiskUsagePercent,
			GPULoad:          m.GPULoad,
			UploadRate:       m.UploadRate,
			DownloadRate:     m.DownloadRate,
		})
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
