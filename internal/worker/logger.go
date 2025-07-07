// Package worker provides background job processing functionality for Docker operations
package worker

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"ontree-node/internal/database"
)

// OperationLogger handles logging for Docker operations
type OperationLogger struct {
	db          *sql.DB
	operationID string
	mu          sync.Mutex
}

// NewOperationLogger creates a new logger for a specific operation
func NewOperationLogger(db *sql.DB, operationID string) *OperationLogger {
	return &OperationLogger{
		db:          db,
		operationID: operationID,
	}
}

// LogDebug logs a debug message
func (ol *OperationLogger) LogDebug(message string, details ...map[string]interface{}) {
	ol.log(database.LogLevelDebug, message, details...)
}

// LogInfo logs an info message
func (ol *OperationLogger) LogInfo(message string, details ...map[string]interface{}) {
	ol.log(database.LogLevelInfo, message, details...)
}

// LogWarning logs a warning message
func (ol *OperationLogger) LogWarning(message string, details ...map[string]interface{}) {
	ol.log(database.LogLevelWarning, message, details...)
}

// LogError logs an error message
func (ol *OperationLogger) LogError(message string, details ...map[string]interface{}) {
	ol.log(database.LogLevelError, message, details...)
}

// LogCommand logs a command that would be executed
func (ol *OperationLogger) LogCommand(description, command string) {
	details := map[string]interface{}{
		"command": command,
		"type":    "equivalent_command",
	}
	ol.LogInfo(description, details)
}

// LogDockerAPI logs a Docker API call
func (ol *OperationLogger) LogDockerAPI(method, endpoint string, statusCode int, responseTime time.Duration) {
	details := map[string]interface{}{
		"method":           method,
		"endpoint":         endpoint,
		"status_code":      statusCode,
		"response_time_ms": responseTime.Milliseconds(),
		"type":             "docker_api",
	}
	message := fmt.Sprintf("Docker API: %s %s", method, endpoint)
	if statusCode > 0 {
		message += fmt.Sprintf(" (%d)", statusCode)
	}
	ol.LogDebug(message, details)
}

// LogProgress logs progress information
func (ol *OperationLogger) LogProgress(message string, progress int, total int) {
	details := map[string]interface{}{
		"progress": progress,
		"total":    total,
		"percent":  float64(progress) / float64(total) * 100,
		"type":     "progress",
	}
	ol.LogInfo(message, details)
}

// log is the internal method that writes to the database
func (ol *OperationLogger) log(level, message string, details ...map[string]interface{}) {
	ol.mu.Lock()
	defer ol.mu.Unlock()

	var detailsJSON sql.NullString
	if len(details) > 0 && details[0] != nil {
		jsonBytes, err := json.Marshal(details[0])
		if err != nil {
			log.Printf("Failed to marshal log details: %v", err)
		} else {
			detailsJSON = sql.NullString{String: string(jsonBytes), Valid: true}
		}
	}

	query := `
		INSERT INTO docker_operation_logs (operation_id, level, message, details)
		VALUES (?, ?, ?, ?)
	`

	_, err := ol.db.Exec(query, ol.operationID, level, message, detailsJSON)
	if err != nil {
		log.Printf("Failed to write operation log: %v", err)
	}

	// Also log to stdout for debugging
	timestamp := time.Now().Format("15:04:05")
	levelStr := fmt.Sprintf("[%s]", level)
	log.Printf("[%s] %s %s", timestamp, levelStr, message)
}

// GetOperationLogs retrieves all logs for an operation
func GetOperationLogs(db *sql.DB, operationID string) ([]database.DockerOperationLog, error) {
	query := `
		SELECT id, operation_id, timestamp, level, message, details
		FROM docker_operation_logs
		WHERE operation_id = ?
		ORDER BY timestamp ASC, id ASC
	`

	rows, err := db.Query(query, operationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	var logs []database.DockerOperationLog
	for rows.Next() {
		var log database.DockerOperationLog
		err := rows.Scan(&log.ID, &log.OperationID, &log.Timestamp, &log.Level, &log.Message, &log.Details)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// CleanupOldLogs removes logs older than the specified duration
func CleanupOldLogs(db *sql.DB, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM docker_operation_logs
		WHERE timestamp < ?
	`

	result, err := db.Exec(query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get affected rows: %v", err)
		affected = 0
	}
	if affected > 0 {
		log.Printf("Cleaned up %d old operation logs", affected)
	}

	return nil
}
