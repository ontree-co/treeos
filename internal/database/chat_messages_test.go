package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) *sql.DB {
	// Create a temporary directory for the test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize the database
	err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Run the migration to create the chat_messages table
	db := GetDB()
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			status_level TEXT NOT NULL CHECK (status_level IN ('OK', 'WARNING', 'CRITICAL')),
			message_summary TEXT NOT NULL,
			message_details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create chat_messages table: %v", err)
	}

	// Create index
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_messages_app_id_timestamp ON chat_messages(app_id, timestamp DESC)`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	return db
}

// cleanupTestDB closes and removes the test database
func cleanupTestDB(dbPath string) {
	Close()
	os.Remove(dbPath)
}

func TestCreateChatMessage(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Test creating a chat message with all fields
	msg := ChatMessage{
		AppID:          "test-app",
		Timestamp:      time.Now(),
		StatusLevel:    ChatStatusOK,
		MessageSummary: "All systems nominal",
		MessageDetails: sql.NullString{String: "All services running", Valid: true},
	}

	err := CreateChatMessage(msg)
	if err != nil {
		t.Errorf("Failed to create chat message: %v", err)
	}

	// Test creating a chat message without details
	msg2 := ChatMessage{
		AppID:          "test-app",
		Timestamp:      time.Now(),
		StatusLevel:    ChatStatusWarning,
		MessageSummary: "High memory usage detected",
		MessageDetails: sql.NullString{Valid: false},
	}

	err = CreateChatMessage(msg2)
	if err != nil {
		t.Errorf("Failed to create chat message without details: %v", err)
	}

	// Test creating a chat message with critical status
	msg3 := ChatMessage{
		AppID:          "another-app",
		Timestamp:      time.Now(),
		StatusLevel:    ChatStatusCritical,
		MessageSummary: "Service down",
		MessageDetails: sql.NullString{String: "Database connection failed", Valid: true},
	}

	err = CreateChatMessage(msg3)
	if err != nil {
		t.Errorf("Failed to create critical chat message: %v", err)
	}
}

func TestGetChatMessagesForApp(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Create test messages
	appID := "test-app"
	now := time.Now()

	for i := 0; i < 5; i++ {
		msg := ChatMessage{
			AppID:          appID,
			Timestamp:      now.Add(time.Duration(i) * time.Minute),
			StatusLevel:    ChatStatusOK,
			MessageSummary: "Test message " + string(rune('A'+i)),
		}
		err := CreateChatMessage(msg)
		if err != nil {
			t.Fatalf("Failed to create test message: %v", err)
		}
	}

	// Test getting messages with limit and offset
	messages, err := GetChatMessagesForApp(appID, 3, 0)
	if err != nil {
		t.Errorf("Failed to get chat messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Messages should be ordered by timestamp DESC
	if messages[0].MessageSummary != "Test message E" {
		t.Errorf("Expected most recent message first, got %s", messages[0].MessageSummary)
	}

	// Test pagination
	messages2, err := GetChatMessagesForApp(appID, 2, 3)
	if err != nil {
		t.Errorf("Failed to get paginated messages: %v", err)
	}

	if len(messages2) != 2 {
		t.Errorf("Expected 2 messages with offset, got %d", len(messages2))
	}

	// Test getting messages for non-existent app
	messages3, err := GetChatMessagesForApp("non-existent", 10, 0)
	if err != nil {
		t.Errorf("Failed to get messages for non-existent app: %v", err)
	}

	if len(messages3) != 0 {
		t.Errorf("Expected 0 messages for non-existent app, got %d", len(messages3))
	}
}

func TestGetLatestChatMessageForApp(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	appID := "test-app"

	// Test when no messages exist
	msg, err := GetLatestChatMessageForApp(appID)
	if err != nil {
		t.Errorf("Failed to get latest message: %v", err)
	}
	if msg != nil {
		t.Errorf("Expected nil for non-existent messages, got %v", msg)
	}

	// Create messages with different timestamps
	now := time.Now()
	for i := 0; i < 3; i++ {
		msg := ChatMessage{
			AppID:          appID,
			Timestamp:      now.Add(time.Duration(i) * time.Hour),
			StatusLevel:    ChatStatusOK,
			MessageSummary: "Message " + string(rune('A'+i)),
		}
		err := CreateChatMessage(msg)
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	// Get latest message
	latest, err := GetLatestChatMessageForApp(appID)
	if err != nil {
		t.Errorf("Failed to get latest message: %v", err)
	}

	if latest == nil {
		t.Errorf("Expected a message, got nil")
	} else if latest.MessageSummary != "Message C" {
		t.Errorf("Expected latest message 'Message C', got %s", latest.MessageSummary)
	}
}

func TestCountChatMessagesForApp(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	appID := "test-app"

	// Test count for empty app
	count, err := CountChatMessagesForApp(appID)
	if err != nil {
		t.Errorf("Failed to count messages: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 messages, got %d", count)
	}

	// Create messages
	for i := 0; i < 7; i++ {
		msg := ChatMessage{
			AppID:          appID,
			Timestamp:      time.Now(),
			StatusLevel:    ChatStatusOK,
			MessageSummary: "Test message",
		}
		err := CreateChatMessage(msg)
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	// Test count after creating messages
	count, err = CountChatMessagesForApp(appID)
	if err != nil {
		t.Errorf("Failed to count messages: %v", err)
	}
	if count != 7 {
		t.Errorf("Expected 7 messages, got %d", count)
	}

	// Create messages for another app
	for i := 0; i < 3; i++ {
		msg := ChatMessage{
			AppID:          "another-app",
			Timestamp:      time.Now(),
			StatusLevel:    ChatStatusOK,
			MessageSummary: "Another message",
		}
		err := CreateChatMessage(msg)
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	// Verify count is still correct for first app
	count, err = CountChatMessagesForApp(appID)
	if err != nil {
		t.Errorf("Failed to count messages: %v", err)
	}
	if count != 7 {
		t.Errorf("Expected 7 messages for first app, got %d", count)
	}
}

func TestDeleteOldChatMessages(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	appID := "test-app"
	now := time.Now()

	// Create old messages (8 days ago)
	for i := 0; i < 3; i++ {
		oldTime := now.Add(-8 * 24 * time.Hour)
		_, err := GetDB().Exec(`
			INSERT INTO chat_messages (app_id, timestamp, status_level, message_summary)
			VALUES (?, ?, ?, ?)
		`, appID, oldTime, ChatStatusOK, "Old message")
		if err != nil {
			t.Fatalf("Failed to create old message: %v", err)
		}
	}

	// Create recent messages (1 day ago)
	for i := 0; i < 2; i++ {
		recentTime := now.Add(-24 * time.Hour)
		_, err := GetDB().Exec(`
			INSERT INTO chat_messages (app_id, timestamp, status_level, message_summary)
			VALUES (?, ?, ?, ?)
		`, appID, recentTime, ChatStatusOK, "Recent message")
		if err != nil {
			t.Fatalf("Failed to create recent message: %v", err)
		}
	}

	// Delete messages older than 7 days
	deleted, err := DeleteOldChatMessages(7 * 24 * time.Hour)
	if err != nil {
		t.Errorf("Failed to delete old messages: %v", err)
	}

	if deleted != 3 {
		t.Errorf("Expected 3 deleted messages, got %d", deleted)
	}

	// Verify only recent messages remain
	count, err := CountChatMessagesForApp(appID)
	if err != nil {
		t.Errorf("Failed to count remaining messages: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 remaining messages, got %d", count)
	}
}
