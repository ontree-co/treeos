package database

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateChatMessage creates a new chat message in the database.
func CreateChatMessage(message ChatMessage) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		INSERT INTO chat_messages (app_id, timestamp, status_level, message_summary, message_details)
		VALUES (?, ?, ?, ?, ?)
	`

	var messageDetails interface{}
	if message.MessageDetails.Valid {
		messageDetails = message.MessageDetails.String
	} else {
		messageDetails = nil
	}

	timestamp := message.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	_, err := db.Exec(query, message.AppID, timestamp, message.StatusLevel, message.MessageSummary, messageDetails)
	if err != nil {
		return fmt.Errorf("failed to create chat message: %w", err)
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
		SELECT id, app_id, timestamp, status_level, message_summary, message_details, created_at
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
			&msg.StatusLevel,
			&msg.MessageSummary,
			&msg.MessageDetails,
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
		SELECT id, app_id, timestamp, status_level, message_summary, message_details, created_at
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
		&msg.StatusLevel,
		&msg.MessageSummary,
		&msg.MessageDetails,
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
