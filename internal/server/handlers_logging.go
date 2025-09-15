package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogEntry represents a log entry from the browser
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// handleBrowserLog receives logs from the browser and writes them to a file (development only)
func (s *Server) handleBrowserLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var entry LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Add timestamp if not provided
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format("2006/01/02 15:04:05")
	}

	// Ensure source is set
	if entry.Source == "" {
		entry.Source = "browser"
	}

	// Check if we're in development mode
	isDevelopment := os.Getenv("TREEOS_ENV") == "development" || os.Getenv("DEBUG") == "true"

	if !isDevelopment {
		// In production, just log to stdout (for PostHog/monitoring)
		log.Printf("[BROWSER] [%s] %s", strings.ToUpper(entry.Level), entry.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	// In development, write to unified log file
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Failed to create log directory: %v", err)
		// Don't fail the request, just log to server logs
	}

	// Write to the SAME file as server logs (treeos.log)
	logPath := filepath.Join(logDir, "treeos.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		// Log to stdout as fallback
		log.Printf("[BROWSER] [%s] %s", strings.ToUpper(entry.Level), entry.Message)
	} else {
		defer file.Close()

		// Format log entry to match server log format
		var logLine string
		if len(entry.Details) > 0 {
			detailsJSON, _ := json.Marshal(entry.Details)
			logLine = fmt.Sprintf("%s [%s] [BROWSER] %s %s\n",
				entry.Timestamp, strings.ToUpper(entry.Level), entry.Message, string(detailsJSON))
		} else {
			logLine = fmt.Sprintf("%s [%s] [BROWSER] %s\n",
				entry.Timestamp, strings.ToUpper(entry.Level), entry.Message)
		}
		file.WriteString(logLine)

		// Also log to stdout in development for immediate visibility
		log.Printf("[BROWSER] [%s] %s", strings.ToUpper(entry.Level), entry.Message)
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleGetLogs queries Loki for recent logs or falls back to reading log files directly
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	// Query parameters
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "100"
	}

	source := r.URL.Query().Get("source") // "server", "browser", or empty for all

	// First, try to query Loki API if available
	lokiAvailable := s.checkLokiAvailability()

	if lokiAvailable {
		// Build Loki query based on source
		var query string
		switch source {
		case "server":
			query = "{source=\"server\"}"
		case "browser":
			query = "{source=\"browser\"}"
		default:
			query = "{job=\"treeos\"}"
		}

		// Query Loki API
		lokiURL := fmt.Sprintf("http://localhost:3100/loki/api/v1/query_range?query=%s&limit=%s", query, limit)

		resp, err := http.Get(lokiURL)
		if err != nil {
			log.Printf("Failed to query Loki: %v", err)
			// Fall back to file reading
			s.sendLogsFromFiles(w, source, limit)
			return
		}
		defer resp.Body.Close()

		// Forward response
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	} else {
		// Loki not available, read from files
		s.sendLogsFromFiles(w, source, limit)
	}
}

// checkLokiAvailability checks if Loki is running and accessible
func (s *Server) checkLokiAvailability() bool {
	resp, err := http.Get("http://localhost:3100/ready")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// sendLogsFromFiles reads logs directly from files when Loki is not available
func (s *Server) sendLogsFromFiles(w http.ResponseWriter, source string, limit string) {
	// Use appropriate log directory based on environment
	isDevelopment := os.Getenv("TREEOS_ENV") == "development" || os.Getenv("DEBUG") == "true"
	logDir := "./logs"
	if !isDevelopment {
		logDir = "/opt/ontree/logs" // Keep production path for backward compatibility
	}
	var logs []map[string]interface{}

	// Helper function to read last N lines from a file
	readLastLines := func(filepath string, n int) []string {
		file, err := os.Open(filepath)
		if err != nil {
			return []string{}
		}
		defer file.Close()

		// This is a simple implementation - for production, use a more efficient approach
		var lines []string
		content, err := io.ReadAll(file)
		if err != nil {
			return []string{}
		}

		allLines := string(content)
		for _, line := range splitLines(allLines) {
			if line != "" {
				lines = append(lines, line)
			}
		}

		// Return last n lines
		if len(lines) > n {
			return lines[len(lines)-n:]
		}
		return lines
	}

	maxLines := 100
	fmt.Sscanf(limit, "%d", &maxLines)

	// In development, all logs are in treeos.log
	// In production, they might be separate (if file logging is enabled)
	if isDevelopment {
		// All logs are in the unified file
		unifiedLogPath := filepath.Join(logDir, "treeos.log")
		logLines := readLastLines(unifiedLogPath, maxLines)
		for _, line := range logLines {
			// Determine source from log line content
			logSource := "server"
			if strings.Contains(line, "[BROWSER]") {
				logSource = "browser"
			}

			// Filter by requested source if specified
			if source == "" || source == logSource {
				logs = append(logs, map[string]interface{}{
					"source":  logSource,
					"message": line,
					"time":    time.Now().Format(time.RFC3339),
				})
			}
		}
	} else {
		// Production mode - keep backward compatibility with separate files
		// Read server logs
		if source == "" || source == "server" {
			serverLogPath := filepath.Join(logDir, "treeos.log")
			serverLines := readLastLines(serverLogPath, maxLines)
			for _, line := range serverLines {
				logs = append(logs, map[string]interface{}{
					"source":  "server",
					"message": line,
					"time":    time.Now().Format(time.RFC3339),
				})
			}
		}

		// Read browser logs (if they exist in production)
		if source == "" || source == "browser" {
			browserLogPath := filepath.Join(logDir, "browser.log")
			browserLines := readLastLines(browserLogPath, maxLines)
			for _, line := range browserLines {
				logs = append(logs, map[string]interface{}{
					"source":  "browser",
					"message": line,
					"time":    time.Now().Format(time.RFC3339),
				})
			}
		}
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"result": logs,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}