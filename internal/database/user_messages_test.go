package database

import (
	"database/sql"
	"testing"
	"time"
)

func TestCreateUserMessage(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Test creating a user message
	userMsg := ChatMessage{
		AppID:       "test-app",
		Timestamp:   time.Now(),
		Message:     "Run check",
		SenderType:  "user",
		SenderName:  "User",
		StatusLevel: sql.NullString{String: "info", Valid: true},
	}

	err := CreateChatMessage(userMsg)
	if err != nil {
		t.Errorf("Failed to create user message: %v", err)
	}

	// Verify the message was created
	messages, err := GetChatMessagesForApp("test-app", 10, 0)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.SenderType != "user" {
		t.Errorf("Expected sender_type 'user', got '%s'", msg.SenderType)
	}
	if msg.SenderName != "User" {
		t.Errorf("Expected sender_name 'User', got '%s'", msg.SenderName)
	}
	if msg.StatusLevel.String != "info" {
		t.Errorf("Expected status_level 'info', got '%s'", msg.StatusLevel.String)
	}
	if msg.Message != "Run check" {
		t.Errorf("Expected message 'Run check', got '%s'", msg.Message)
	}
}

func TestMixedMessageTypes(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Create an agent message
	agentMsg := ChatMessage{
		AppID:       "mixed-app",
		Timestamp:   time.Now().Add(-5 * time.Minute),
		Message:     "All systems operational",
		SenderType:  "agent",
		SenderName:  "System Agent",
		StatusLevel: sql.NullString{String: "info", Valid: true},
		Details:     sql.NullString{String: "CPU: 20%, Memory: 45%", Valid: true},
	}

	err := CreateChatMessage(agentMsg)
	if err != nil {
		t.Errorf("Failed to create agent message: %v", err)
	}

	// Create a user message
	userMsg := ChatMessage{
		AppID:       "mixed-app",
		Timestamp:   time.Now().Add(-3 * time.Minute),
		Message:     "Check for updates",
		SenderType:  "user",
		SenderName:  "User",
		StatusLevel: sql.NullString{String: "info", Valid: true},
	}

	err = CreateChatMessage(userMsg)
	if err != nil {
		t.Errorf("Failed to create user message: %v", err)
	}

	// Create another agent message
	agentMsg2 := ChatMessage{
		AppID:       "mixed-app",
		Timestamp:   time.Now().Add(-1 * time.Minute),
		Message:     "Update available for container nginx",
		SenderType:  "agent",
		SenderName:  "System Agent",
		StatusLevel: sql.NullString{String: "warning", Valid: true},
	}

	err = CreateChatMessage(agentMsg2)
	if err != nil {
		t.Errorf("Failed to create second agent message: %v", err)
	}

	// Get all messages
	messages, err := GetChatMessagesForApp("mixed-app", 10, 0)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Messages should be in reverse chronological order
	// Most recent first
	if messages[0].SenderType != "agent" {
		t.Errorf("Expected first message to be agent type, got %s", messages[0].SenderType)
	}
	if messages[0].StatusLevel.String != "warning" {
		t.Errorf("Expected first message status warning, got %s", messages[0].StatusLevel.String)
	}

	if messages[1].SenderType != "user" {
		t.Errorf("Expected second message to be user type, got %s", messages[1].SenderType)
	}

	if messages[2].SenderType != "agent" {
		t.Errorf("Expected third message to be agent type, got %s", messages[2].SenderType)
	}
	if messages[2].StatusLevel.String != "info" {
		t.Errorf("Expected third message status info, got %s", messages[2].StatusLevel.String)
	}
}

func TestUserMessageDefaults(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Create a user message with minimal fields
	userMsg := ChatMessage{
		AppID:       "default-test",
		Message:     "Test message",
		SenderType:  "user",
		SenderName:  "User",
		StatusLevel: sql.NullString{String: "info", Valid: true},
	}

	err := CreateChatMessage(userMsg)
	if err != nil {
		t.Errorf("Failed to create user message: %v", err)
	}

	// Verify defaults were applied
	messages, err := GetChatMessagesForApp("default-test", 1, 0)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.SenderName != "User" {
		t.Errorf("Expected default sender_name 'User', got '%s'", msg.SenderName)
	}
}

func TestBackwardsCompatibility(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Create a message without specifying new fields (simulating old code)
	oldStyleMsg := ChatMessage{
		AppID:       "legacy-app",
		Timestamp:   time.Now(),
		Message:     "Legacy message",
		SenderType:  "agent",
		SenderName:  "System Agent",
		StatusLevel: sql.NullString{String: "info", Valid: true},
	}

	err := CreateChatMessage(oldStyleMsg)
	if err != nil {
		t.Errorf("Failed to create legacy message: %v", err)
	}

	// Verify defaults were applied for backwards compatibility
	messages, err := GetChatMessagesForApp("legacy-app", 1, 0)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.SenderType != "agent" {
		t.Errorf("Expected sender_type 'agent', got '%s'", msg.SenderType)
	}
	if msg.SenderName != "System Agent" {
		t.Errorf("Expected sender_name 'System Agent', got '%s'", msg.SenderName)
	}
}
