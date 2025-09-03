package agent

import (
	"strings"
	"testing"
	"time"

	"ontree-node/internal/database"
)

// TestOrchestratorRunCheck is skipped for now since it requires full integration
// The orchestrator is tested through integration tests instead
func TestOrchestratorRunCheckSkipped(t *testing.T) {
	t.Skip("Skipping integration test - requires full setup")
}

func TestOrchestratorFallbackResponse(t *testing.T) {
	// Initialize test database
	testDB := ":memory:"
	if err := database.Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	defer database.Close()

	// Create test configs
	testConfigs := []AppConfig{
		{
			ID:               "nextcloud",
			Name:             "Nextcloud",
			PrimaryService:   "app",
			ExpectedServices: []string{"app", "db"},
		},
	}

	// Create test snapshot with a failed service
	testSnapshot := &SystemSnapshot{
		Timestamp: time.Now(),
		ServerHealth: ServerHealth{
			CPUUsagePercent:    25.5,
			MemoryUsagePercent: 45.2,
			DiskUsagePercent:   60.1,
		},
		AppStatuses: []AppStatus{
			{
				AppName: "Nextcloud",
				DesiredState: DesiredState{
					ExpectedServices: []string{"app", "db"},
				},
				ActualState: ActualState{
					UptimeKumaStatus: UptimeKumaStatusUp,
					Services: []ServiceStatus{
						{
							Name:         "app",
							Status:       ServiceStatusRunning,
							RestartCount: 0,
						},
						{
							Name:         "db",
							Status:       ServiceStatusExited,
							RestartCount: 0,
						},
					},
				},
			},
		},
	}

	orchestrator := &Orchestrator{}

	// Test fallback response generation
	response := orchestrator.createFallbackResponse(testSnapshot, testConfigs)

	if response.OverallStatus != StatusCritical {
		t.Errorf("Expected overall status %s, got %s", StatusCritical, response.OverallStatus)
	}

	if len(response.RecommendedActions) != 1 {
		t.Errorf("Expected 1 recommended action, got %d", len(response.RecommendedActions))
	}

	if len(response.RecommendedActions) > 0 {
		action := response.RecommendedActions[0]
		if action.ActionKey != ActionPersistChatMessage {
			t.Errorf("Expected action key %s, got %s", ActionPersistChatMessage, action.ActionKey)
		}

		status, ok := action.Parameters["status"].(string)
		if !ok || status != "critical" {
			t.Errorf("Expected critical status in action parameters")
		}

		message, ok := action.Parameters["message"].(string)
		if !ok || message != "Service db is exited" {
			t.Errorf("Expected error message about db service, got: %s", message)
		}
	}
}

func TestExecutePersistChatMessage(t *testing.T) {
	// Initialize test database
	testDB := ":memory:"
	if err := database.Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	defer database.Close()

	orchestrator := &Orchestrator{}

	// Test action with valid parameters
	action := RecommendedAction{
		ActionKey: ActionPersistChatMessage,
		Parameters: map[string]interface{}{
			"app_id":  "test-app",
			"status":  "warning",
			"message": "Test warning message",
		},
		Justification: "Test justification",
	}

	err := orchestrator.executePersistChatMessage(action)
	if err != nil {
		t.Errorf("executePersistChatMessage failed: %v", err)
	}

	// Verify message was persisted
	messages, err := database.GetChatMessagesForApp("test-app", 10, 0)
	if err != nil {
		t.Errorf("Failed to get chat messages: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if len(messages) > 0 {
		msg := messages[0]
		if msg.AppID != "test-app" {
			t.Errorf("Expected app_id 'test-app', got '%s'", msg.AppID)
		}
		if !msg.StatusLevel.Valid || msg.StatusLevel.String != database.StatusLevelWarning {
			t.Errorf("Expected status '%s', got '%s'", database.StatusLevelWarning, msg.StatusLevel.String)
		}
		if msg.Message != "Test warning message" {
			t.Errorf("Expected message 'Test warning message', got '%s'", msg.Message)
		}

		// Check if justification was stored in details
		if !msg.Details.Valid || msg.Details.String != "Test justification" {
			t.Errorf("Expected justification 'Test justification' in details, got '%s'", msg.Details.String)
		}
	}
}

func TestExecutePersistChatMessageMissingParams(t *testing.T) {
	orchestrator := &Orchestrator{}

	// Test with missing app_id
	action := RecommendedAction{
		ActionKey: ActionPersistChatMessage,
		Parameters: map[string]interface{}{
			"status":  "info",
			"message": "Test message",
		},
	}

	err := orchestrator.executePersistChatMessage(action)
	if err == nil {
		t.Error("Expected error for missing app_id parameter")
	}

	// Test with missing status
	action = RecommendedAction{
		ActionKey: ActionPersistChatMessage,
		Parameters: map[string]interface{}{
			"app_id":  "test-app",
			"message": "Test message",
		},
	}

	err = orchestrator.executePersistChatMessage(action)
	if err == nil {
		t.Error("Expected error for missing status parameter")
	}

	// Test with missing message
	action = RecommendedAction{
		ActionKey: ActionPersistChatMessage,
		Parameters: map[string]interface{}{
			"app_id": "test-app",
			"status": "info",
		},
	}

	err = orchestrator.executePersistChatMessage(action)
	if err == nil {
		t.Error("Expected error for missing message parameter")
	}
}

func TestCreateFallbackResponseHighRestartCount(t *testing.T) {
	testConfigs := []AppConfig{
		{
			ID:               "app1",
			Name:             "App One",
			PrimaryService:   "main",
			ExpectedServices: []string{"main"},
		},
	}

	testSnapshot := &SystemSnapshot{
		Timestamp: time.Now(),
		ServerHealth: ServerHealth{
			CPUUsagePercent:    10.0,
			MemoryUsagePercent: 20.0,
			DiskUsagePercent:   30.0,
		},
		AppStatuses: []AppStatus{
			{
				AppName: "App One",
				ActualState: ActualState{
					Services: []ServiceStatus{
						{
							Name:         "main",
							Status:       ServiceStatusRunning,
							RestartCount: 10, // High restart count
						},
					},
				},
			},
		},
	}

	orchestrator := &Orchestrator{}
	response := orchestrator.createFallbackResponse(testSnapshot, testConfigs)

	if response.OverallStatus != StatusWarning {
		t.Errorf("Expected warning status for high restart count, got %s", response.OverallStatus)
	}

	// Check that appropriate action was recommended
	if len(response.RecommendedActions) != 1 {
		t.Errorf("Expected 1 recommended action, got %d", len(response.RecommendedActions))
	}

	if len(response.RecommendedActions) > 0 {
		action := response.RecommendedActions[0]
		status := action.Parameters["status"].(string)
		if status != "warning" {
			t.Errorf("Expected warning status in action, got %s", status)
		}

		message := action.Parameters["message"].(string)
		if !strings.Contains(message, "restart count") {
			t.Errorf("Expected message about restart count, got: %s", message)
		}
	}
}
