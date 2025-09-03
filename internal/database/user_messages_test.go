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
		AppID:          "test-app",
		Timestamp:      time.Now(),
		StatusLevel:    ChatStatusUser,
		MessageSummary: "Run check",
		MessageType:    ChatMessageTypeUser,
		SenderName:     "User",
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
	if msg.MessageType != ChatMessageTypeUser {
		t.Errorf("Expected message_type 'user', got '%s'", msg.MessageType)
	}
	if msg.SenderName != "User" {
		t.Errorf("Expected sender_name 'User', got '%s'", msg.SenderName)
	}
	if msg.StatusLevel != ChatStatusUser {
		t.Errorf("Expected status_level 'USER', got '%s'", msg.StatusLevel)
	}
	if msg.MessageSummary != "Run check" {
		t.Errorf("Expected message_summary 'Run check', got '%s'", msg.MessageSummary)
	}
}

func TestMixedMessageTypes(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Create an agent message
	agentMsg := ChatMessage{
		AppID:          "mixed-app",
		Timestamp:      time.Now().Add(-5 * time.Minute),
		StatusLevel:    ChatStatusOK,
		MessageSummary: "All systems operational",
		MessageDetails: sql.NullString{String: "CPU: 20%, Memory: 45%", Valid: true},
		MessageType:    ChatMessageTypeAgent,
		SenderName:     "System Agent",
	}

	err := CreateChatMessage(agentMsg)
	if err != nil {
		t.Errorf("Failed to create agent message: %v", err)
	}

	// Create a user message
	userMsg := ChatMessage{
		AppID:          "mixed-app",
		Timestamp:      time.Now().Add(-3 * time.Minute),
		StatusLevel:    ChatStatusUser,
		MessageSummary: "Check for updates",
		MessageType:    ChatMessageTypeUser,
		SenderName:     "User",
	}

	err = CreateChatMessage(userMsg)
	if err != nil {
		t.Errorf("Failed to create user message: %v", err)
	}

	// Create another agent message
	agentMsg2 := ChatMessage{
		AppID:          "mixed-app",
		Timestamp:      time.Now().Add(-1 * time.Minute),
		StatusLevel:    ChatStatusWarning,
		MessageSummary: "Update available for container nginx",
		MessageType:    ChatMessageTypeAgent,
		SenderName:     "System Agent",
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
	if messages[0].MessageType != ChatMessageTypeAgent {
		t.Errorf("Expected first message to be agent type, got %s", messages[0].MessageType)
	}
	if messages[0].StatusLevel != ChatStatusWarning {
		t.Errorf("Expected first message status WARNING, got %s", messages[0].StatusLevel)
	}

	if messages[1].MessageType != ChatMessageTypeUser {
		t.Errorf("Expected second message to be user type, got %s", messages[1].MessageType)
	}

	if messages[2].MessageType != ChatMessageTypeAgent {
		t.Errorf("Expected third message to be agent type, got %s", messages[2].MessageType)
	}
	if messages[2].StatusLevel != ChatStatusOK {
		t.Errorf("Expected third message status OK, got %s", messages[2].StatusLevel)
	}
}

func TestUserMessageDefaults(t *testing.T) {
	_ = setupTestDB(t)
	defer Close()

	// Create a user message with minimal fields
	userMsg := ChatMessage{
		AppID:          "default-test",
		StatusLevel:    ChatStatusUser,
		MessageSummary: "Test message",
		MessageType:    ChatMessageTypeUser,
		// SenderName should default to "User"
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
		AppID:          "legacy-app",
		Timestamp:      time.Now(),
		StatusLevel:    ChatStatusOK,
		MessageSummary: "Legacy message",
		// MessageType and SenderName not specified
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
	if msg.MessageType != ChatMessageTypeAgent {
		t.Errorf("Expected default message_type 'agent', got '%s'", msg.MessageType)
	}
	if msg.SenderName != "System Agent" {
		t.Errorf("Expected default sender_name 'System Agent', got '%s'", msg.SenderName)
	}
}