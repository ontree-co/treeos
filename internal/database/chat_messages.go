package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ChatMessageCallback is a function that gets called when a new chat message is created
var ChatMessageCallback func(message ChatMessage)

// CreateChatMessage creates a new chat message in the database.
func CreateChatMessage(message ChatMessage) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		INSERT INTO chat_messages (
			app_id, timestamp, message, sender_type, sender_name,
			agent_model, agent_provider, status_level, details
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	timestamp := message.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	result, err := db.Exec(query,
		message.AppID,
		timestamp,
		message.Message,
		message.SenderType,
		message.SenderName,
		nullableString(message.AgentModel),
		nullableString(message.AgentProvider),
		nullableString(message.StatusLevel),
		nullableString(message.Details),
	)
	if err != nil {
		return fmt.Errorf("failed to create chat message: %w", err)
	}

	// Get the inserted ID
	id, err := result.LastInsertId()
	if err == nil {
		message.ID = int(id)
		message.Timestamp = timestamp
	}

	// Call the callback if set (for SSE broadcasting)
	if ChatMessageCallback != nil {
		ChatMessageCallback(message)
	}

	return nil
}

// Helper function to handle nullable strings
func nullableString(ns sql.NullString) interface{} {
	if ns.Valid {
		return ns.String
	}
	return nil
}

// GetChatMessagesForApp retrieves chat messages for a specific application.
func GetChatMessagesForApp(appID string, limit int, offset int) ([]ChatMessage, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, app_id, timestamp, message, sender_type, sender_name,
		       agent_model, agent_provider, status_level, details, created_at
		FROM chat_messages
		WHERE app_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := db.Query(query, appID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat messages: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(
			&msg.ID,
			&msg.AppID,
			&msg.Timestamp,
			&msg.Message,
			&msg.SenderType,
			&msg.SenderName,
			&msg.AgentModel,
			&msg.AgentProvider,
			&msg.StatusLevel,
			&msg.Details,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chat messages: %w", err)
	}

	return messages, nil
}

// GetLatestChatMessageForApp retrieves the most recent chat message for an application.
func GetLatestChatMessageForApp(appID string) (*ChatMessage, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, app_id, timestamp, message, sender_type, sender_name,
		       agent_model, agent_provider, status_level, details, created_at
		FROM chat_messages
		WHERE app_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var msg ChatMessage
	err := db.QueryRow(query, appID).Scan(
		&msg.ID,
		&msg.AppID,
		&msg.Timestamp,
		&msg.Message,
		&msg.SenderType,
		&msg.SenderName,
		&msg.AgentModel,
		&msg.AgentProvider,
		&msg.StatusLevel,
		&msg.Details,
		&msg.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest chat message: %w", err)
	}

	return &msg, nil
}

// CountChatMessagesForApp returns the total count of chat messages for an application.
func CountChatMessagesForApp(appID string) (int, error) {
	db := GetDB()
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	query := `SELECT COUNT(*) FROM chat_messages WHERE app_id = ?`

	var count int
	err := db.QueryRow(query, appID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count chat messages: %w", err)
	}

	return count, nil
}

// DeleteOldChatMessages deletes chat messages older than the specified duration.
func DeleteOldChatMessages(olderThan time.Duration) (int64, error) {
	db := GetDB()
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	cutoffTime := time.Now().Add(-olderThan)
	query := `DELETE FROM chat_messages WHERE timestamp < ?`

	result, err := db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old chat messages: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}
