package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"ontree-node/internal/database"
	"ontree-node/internal/worker"
)

// createDockerOperation creates a new Docker operation record in the database
func (s *Server) createDockerOperation(operationType, appName string, metadata map[string]string) (string, error) {
	operationID := uuid.New().String()

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO docker_operations (id, operation_type, app_name, status, metadata)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		operationID,
		operationType,
		appName,
		database.StatusPending,
		string(metadataJSON),
	)

	if err != nil {
		return "", fmt.Errorf("failed to create operation: %w", err)
	}

	return operationID, nil
}

// handleDockerOperationStatus handles the API endpoint for checking operation status
func (s *Server) handleDockerOperationStatus(w http.ResponseWriter, r *http.Request) {
	// Extract operation ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 || parts[1] != "api" || parts[2] != "docker" || parts[3] != "operations" {
		http.NotFound(w, r)
		return
	}

	operationID := parts[4]

	// Get operation details from database
	var op database.DockerOperation
	err := s.db.QueryRow(
		`SELECT id, operation_type, app_name, status, progress, progress_message, 
		        error_message, created_at, updated_at, completed_at
		 FROM docker_operations WHERE id = ?`,
		operationID,
	).Scan(
		&op.ID,
		&op.OperationType,
		&op.AppName,
		&op.Status,
		&op.Progress,
		&op.ProgressMessage,
		&op.ErrorMessage,
		&op.CreatedAt,
		&op.UpdatedAt,
		&op.CompletedAt,
	)

	if err != nil {
		log.Printf("Failed to get operation %s: %v", operationID, err)
		http.Error(w, "Operation not found", http.StatusNotFound)
		return
	}

	// Check if client wants JSON or HTML response
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		// Return JSON for programmatic access
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(op)
		return
	}

	// Return HTML fragment for HTMX
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Generate appropriate HTML based on status
	var html string
	switch op.Status {
	case database.StatusPending:
		html = fmt.Sprintf(`
			<div class="operation-status" hx-get="/api/docker/operations/%s" hx-trigger="load delay:1s" hx-swap="outerHTML">
				<div class="spinner-border spinner-border-sm me-2" role="status">
					<span class="visually-hidden">Loading...</span>
				</div>
				<span>Waiting to start...</span>
			</div>
		`, operationID)

	case database.StatusInProgress:
		progressBar := ""
		if op.Progress > 0 {
			progressBar = fmt.Sprintf(`
				<div class="progress mt-2" style="height: 20px;">
					<div class="progress-bar progress-bar-striped progress-bar-animated" 
					     role="progressbar" style="width: %d%%;" 
					     aria-valuenow="%d" aria-valuemin="0" aria-valuemax="100">%d%%</div>
				</div>
			`, op.Progress, op.Progress, op.Progress)
		}

		html = fmt.Sprintf(`
			<div class="operation-status" hx-get="/api/docker/operations/%s" hx-trigger="load delay:1s" hx-swap="outerHTML">
				<div class="spinner-border spinner-border-sm me-2" role="status">
					<span class="visually-hidden">Loading...</span>
				</div>
				<span>%s</span>
				%s
			</div>
		`, operationID, op.ProgressMessage.String, progressBar)

	case database.StatusCompleted:
		// Calculate duration
		duration := ""
		if op.CompletedAt.Valid && !op.CompletedAt.Time.IsZero() {
			dur := op.CompletedAt.Time.Sub(op.CreatedAt)
			duration = fmt.Sprintf(" (took %s)", dur.Round(time.Second))
		}

		html = fmt.Sprintf(`
			<div class="operation-status text-success">
				<i class="bi bi-check-circle-fill me-2"></i>
				<span>%s%s</span>
			</div>
		`, op.ProgressMessage.String, duration)

	case database.StatusFailed:
		html = fmt.Sprintf(`
			<div class="operation-status text-danger">
				<i class="bi bi-x-circle-fill me-2"></i>
				<span>Failed: %s</span>
			</div>
		`, op.ErrorMessage.String)

	default:
		html = fmt.Sprintf(`
			<div class="operation-status text-muted">
				<span>Unknown status: %s</span>
			</div>
		`, op.Status)
	}

	w.Write([]byte(html))
}

// handleDockerOperationLogs handles the API endpoint for retrieving operation logs
func (s *Server) handleDockerOperationLogs(w http.ResponseWriter, r *http.Request) {
	// Extract operation ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 6 || parts[1] != "api" || parts[2] != "docker" || parts[3] != "operations" || parts[5] != "logs" {
		http.NotFound(w, r)
		return
	}

	operationID := parts[4]

	// Get logs from database
	logs, err := worker.GetOperationLogs(s.db, operationID)
	if err != nil {
		log.Printf("Failed to get operation logs: %v", err)
		http.Error(w, "Failed to retrieve logs", http.StatusInternalServerError)
		return
	}

	// Check if client wants JSON or HTML response
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		// Return JSON for programmatic access
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
		return
	}

	// Return HTML fragment for HTMX
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Generate HTML for logs
	var html strings.Builder
	html.WriteString(`<div class="logs-content" hx-get="/api/docker/operations/`)
	html.WriteString(operationID)
	html.WriteString(`/logs" hx-trigger="load delay:2s" hx-swap="innerHTML">`)

	if len(logs) == 0 {
		html.WriteString(`<div class="text-muted">No logs available yet...</div>`)
	} else {
		for _, log := range logs {
			// Determine CSS class based on log level
			levelClass := "log-" + log.Level
			
			html.WriteString(`<div class="log-entry `)
			html.WriteString(levelClass)
			html.WriteString(`">`)
			
			// Timestamp
			html.WriteString(`<span class="timestamp">[`)
			html.WriteString(log.Timestamp.Format("15:04:05"))
			html.WriteString(`]</span> `)
			
			// Level
			html.WriteString(`<span class="level">[`)
			html.WriteString(strings.ToUpper(log.Level))
			html.WriteString(`]</span> `)
			
			// Message
			html.WriteString(`<span class="message">`)
			html.WriteString(template.HTMLEscapeString(log.Message))
			html.WriteString(`</span>`)
			
			// Details (if any)
			if log.Details.Valid && log.Details.String != "" {
				var details map[string]interface{}
				if err := json.Unmarshal([]byte(log.Details.String), &details); err == nil {
					if cmd, ok := details["command"].(string); ok {
						html.WriteString(`<div class="details">`)
						html.WriteString(`<code>$ `)
						html.WriteString(template.HTMLEscapeString(cmd))
						html.WriteString(`</code></div>`)
					}
				}
			}
			
			html.WriteString(`</div>`)
		}
	}

	// Check if operation is still in progress to continue polling
	var status string
	err = s.db.QueryRow("SELECT status FROM docker_operations WHERE id = ?", operationID).Scan(&status)
	if err == nil && (status == database.StatusPending || status == database.StatusInProgress) {
		// Keep polling if operation is still active
		html.WriteString(`</div>`)
	} else {
		// Stop polling when operation is complete
		html.WriteString(`</div><!-- Operation complete, polling stopped -->`)
	}

	w.Write([]byte(html.String()))
}
