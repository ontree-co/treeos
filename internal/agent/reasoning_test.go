package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNewReasoningService tests the creation of a new ReasoningService
func TestNewReasoningService(t *testing.T) {
	tests := []struct {
		name    string
		config  LLMConfig
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			config: LLMConfig{
				APIKey: "test-key",
				APIURL: "https://api.example.com/v1/chat",
				Model:  "gpt-4",
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults",
			config: LLMConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: LLMConfig{
				APIURL: "https://api.example.com/v1/chat",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, err := NewReasoningService(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewReasoningService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && rs == nil {
				t.Error("NewReasoningService() returned nil service")
			}

			// Check defaults were applied
			if !tt.wantErr && tt.config.APIURL == "" && rs.apiURL != "https://api.openai.com/v1/chat/completions" {
				t.Errorf("Default API URL not set correctly: got %s", rs.apiURL)
			}
			if !tt.wantErr && tt.config.Model == "" && rs.model != "gpt-4-turbo-preview" {
				t.Errorf("Default model not set correctly: got %s", rs.model)
			}
		})
	}
}

// TestGeneratePrompt tests prompt generation from SystemSnapshot
func TestGeneratePrompt(t *testing.T) {
	rs := &ReasoningService{}

	snapshot := &SystemSnapshot{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		ServerHealth: ServerHealth{
			CPUUsagePercent:    45.5,
			MemoryUsagePercent: 60.2,
			DiskUsagePercent:   75.0,
		},
		AppStatuses: []AppStatus{
			{
				AppName: "Nextcloud",
				DesiredState: DesiredState{
					ExpectedServices: []string{"app", "db", "redis"},
				},
				ActualState: ActualState{
					UptimeKumaStatus: "UP",
					Services: []ServiceStatus{
						{
							Name:         "app",
							Status:       "running",
							RestartCount: 0,
							LogSummary: LogSummary{
								ErrorsFound:      0,
								SampleErrorLines: []string{},
							},
						},
						{
							Name:         "db",
							Status:       "running",
							RestartCount: 1,
							LogSummary: LogSummary{
								ErrorsFound:      2,
								SampleErrorLines: []string{"ERROR: Connection timeout"},
							},
						},
					},
				},
			},
		},
	}

	prompt, err := rs.GeneratePrompt(snapshot)
	if err != nil {
		t.Fatalf("GeneratePrompt() error = %v", err)
	}

	// Check that prompt contains expected elements
	expectedStrings := []string{
		"The current time is: 2024-01-15T10:30:00Z",
		"\"cpu_usage_percent\": 45.5",
		"\"memory_usage_percent\": 60.2",
		"\"disk_usage_percent\": 75",
		"\"app_name\": \"Nextcloud\"",
		"\"uptime_kuma_status\": \"UP\"",
		"ERROR: Connection timeout",
		"PERSIST_CHAT_MESSAGE",
		"RESTART_CONTAINER",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(prompt, expected) {
			t.Errorf("Generated prompt missing expected string: %s", expected)
		}
	}
}

// TestParseResponse tests parsing of LLM responses
func TestParseResponse(t *testing.T) {
	rs := &ReasoningService{}

	tests := []struct {
		name         string
		responseText string
		wantErr      bool
		validate     func(*LLMResponse) error
	}{
		{
			name: "valid JSON response",
			responseText: `{
				"overall_status": "ALL_OK",
				"summary": "All systems are functioning normally.",
				"analysis": [
					{
						"component": "Server Health",
						"status": "OK",
						"finding": "CPU, memory, and disk usage are within normal ranges."
					}
				],
				"recommended_actions": [
					{
						"action_key": "PERSIST_CHAT_MESSAGE",
						"parameters": {
							"app_id": "nextcloud",
							"status": "OK",
							"message": "All systems nominal"
						},
						"justification": "Regular status update"
					}
				]
			}`,
			wantErr: false,
			validate: func(resp *LLMResponse) error {
				if resp.OverallStatus != "ALL_OK" {
					return fmt.Errorf("expected overall_status ALL_OK, got %s", resp.OverallStatus)
				}
				if len(resp.Analysis) != 1 {
					return fmt.Errorf("expected 1 analysis item, got %d", len(resp.Analysis))
				}
				if len(resp.RecommendedActions) != 1 {
					return fmt.Errorf("expected 1 recommended action, got %d", len(resp.RecommendedActions))
				}
				return nil
			},
		},
		{
			name: "JSON with markdown code blocks",
			responseText: "```json\n" + `{
				"overall_status": "WARNING",
				"summary": "High restart count detected.",
				"analysis": [],
				"recommended_actions": [
					{
						"action_key": "RESTART_CONTAINER",
						"parameters": {"container_name": "nextcloud-db-1"},
						"justification": "Container has high restart count"
					}
				]
			}` + "\n```",
			wantErr: false,
			validate: func(resp *LLMResponse) error {
				if resp.OverallStatus != "WARNING" {
					return fmt.Errorf("expected overall_status WARNING, got %s", resp.OverallStatus)
				}
				return nil
			},
		},
		{
			name: "invalid overall_status",
			responseText: `{
				"overall_status": "INVALID_STATUS",
				"summary": "Test",
				"analysis": [],
				"recommended_actions": []
			}`,
			wantErr: true,
		},
		{
			name: "missing required parameters in action",
			responseText: `{
				"overall_status": "CRITICAL",
				"summary": "Test",
				"analysis": [],
				"recommended_actions": [
					{
						"action_key": "PERSIST_CHAT_MESSAGE",
						"parameters": {"app_id": "test"},
						"justification": "Missing status and message params"
					}
				]
			}`,
			wantErr: true,
		},
		{
			name:         "malformed JSON",
			responseText: `{"overall_status": "ALL_OK", invalid json}`,
			wantErr:      true,
		},
		{
			name:         "empty response",
			responseText: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := rs.parseResponse(tt.responseText)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				if err := tt.validate(response); err != nil {
					t.Errorf("Response validation failed: %v", err)
				}
			}
		})
	}
}

// TestValidateResponse tests response validation
func TestValidateResponse(t *testing.T) {
	rs := &ReasoningService{}

	tests := []struct {
		name     string
		response *LLMResponse
		wantErr  bool
	}{
		{
			name: "valid response",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				Summary:       "All good",
				Analysis: []AnalysisItem{
					{Component: "Test", Status: "OK", Finding: "Fine"},
				},
				RecommendedActions: []RecommendedAction{
					{
						ActionKey: "PERSIST_CHAT_MESSAGE",
						Parameters: map[string]interface{}{
							"app_id":  "test",
							"status":  "OK",
							"message": "Test message",
						},
						Justification: "Test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid overall status",
			response: &LLMResponse{
				OverallStatus: "UNKNOWN",
			},
			wantErr: true,
		},
		{
			name: "invalid analysis status",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				Analysis: []AnalysisItem{
					{Component: "Test", Status: "INVALID", Finding: "Test"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty component in analysis",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				Analysis: []AnalysisItem{
					{Component: "", Status: "OK", Finding: "Test"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid action key",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				RecommendedActions: []RecommendedAction{
					{
						ActionKey:     "INVALID_ACTION",
						Parameters:    map[string]interface{}{},
						Justification: "Test",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing required parameters for PERSIST_CHAT_MESSAGE",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				RecommendedActions: []RecommendedAction{
					{
						ActionKey: "PERSIST_CHAT_MESSAGE",
						Parameters: map[string]interface{}{
							"app_id": "test",
							// missing status and message
						},
						Justification: "Test",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing required parameters for RESTART_CONTAINER",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				RecommendedActions: []RecommendedAction{
					{
						ActionKey:     "RESTART_CONTAINER",
						Parameters:    map[string]interface{}{},
						Justification: "Test",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NO_ACTION with no parameters is valid",
			response: &LLMResponse{
				OverallStatus: "ALL_OK",
				RecommendedActions: []RecommendedAction{
					{
						ActionKey:     "NO_ACTION",
						Parameters:    map[string]interface{}{},
						Justification: "Nothing to do",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rs.validateResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAnalyzeSnapshot tests the full analysis flow with a mock server
func TestAnalyzeSnapshot(t *testing.T) {
	// Create a mock LLM server
	mockResponse := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message": map[string]string{
					"content": `{
						"overall_status": "CRITICAL",
						"summary": "Database service is down for Nextcloud.",
						"analysis": [
							{
								"component": "App: Nextcloud",
								"status": "FAIL",
								"finding": "Database service is not running"
							}
						],
						"recommended_actions": [
							{
								"action_key": "RESTART_CONTAINER",
								"parameters": {"container_name": "nextcloud-db-1"},
								"justification": "Attempt to restart the failed database service"
							},
							{
								"action_key": "PERSIST_CHAT_MESSAGE",
								"parameters": {
									"app_id": "nextcloud",
									"status": "CRITICAL",
									"message": "Database service is down. Attempting restart."
								},
								"justification": "Alert user about critical issue"
							}
						]
					}`,
				},
			},
		},
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Expected Authorization header with Bearer token")
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer mockServer.Close()

	// Create ReasoningService with mock server URL
	rs, err := NewReasoningService(LLMConfig{
		APIKey: "test-key",
		APIURL: mockServer.URL,
		Model:  "test-model",
	})
	if err != nil {
		t.Fatalf("Failed to create ReasoningService: %v", err)
	}

	// Create test snapshot
	snapshot := &SystemSnapshot{
		Timestamp: time.Now(),
		ServerHealth: ServerHealth{
			CPUUsagePercent:    50.0,
			MemoryUsagePercent: 60.0,
			DiskUsagePercent:   70.0,
		},
		AppStatuses: []AppStatus{
			{
				AppName: "Nextcloud",
				DesiredState: DesiredState{
					ExpectedServices: []string{"app", "db"},
				},
				ActualState: ActualState{
					UptimeKumaStatus: "DOWN",
					Services: []ServiceStatus{
						{
							Name:   "app",
							Status: "running",
						},
						{
							Name:   "db",
							Status: "exited",
						},
					},
				},
			},
		},
	}

	// Analyze snapshot
	ctx := context.Background()
	response, err := rs.AnalyzeSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("AnalyzeSnapshot() error = %v", err)
	}

	// Validate response
	if response.OverallStatus != "CRITICAL" {
		t.Errorf("Expected overall_status CRITICAL, got %s", response.OverallStatus)
	}
	if len(response.Analysis) != 1 {
		t.Errorf("Expected 1 analysis item, got %d", len(response.Analysis))
	}
	if len(response.RecommendedActions) != 2 {
		t.Errorf("Expected 2 recommended actions, got %d", len(response.RecommendedActions))
	}

	// Check for specific actions
	hasRestartAction := false
	hasPersistAction := false
	for _, action := range response.RecommendedActions {
		if action.ActionKey == "RESTART_CONTAINER" {
			hasRestartAction = true
			if action.Parameters["container_name"] != "nextcloud-db-1" {
				t.Errorf("Expected container_name nextcloud-db-1, got %v", action.Parameters["container_name"])
			}
		}
		if action.ActionKey == "PERSIST_CHAT_MESSAGE" {
			hasPersistAction = true
			if action.Parameters["app_id"] != "nextcloud" {
				t.Errorf("Expected app_id nextcloud, got %v", action.Parameters["app_id"])
			}
		}
	}

	if !hasRestartAction {
		t.Error("Expected RESTART_CONTAINER action not found")
	}
	if !hasPersistAction {
		t.Error("Expected PERSIST_CHAT_MESSAGE action not found")
	}
}

// TestCallLLMErrorHandling tests error handling in LLM API calls
func TestCallLLMErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name: "HTTP 500 error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			wantErr: true,
		},
		{
			name: "API error in response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"error": map[string]string{
						"message": "API key invalid",
						"type":    "authentication_error",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			wantErr: true,
		},
		{
			name: "Empty choices array",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"choices": []interface{}{},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			wantErr: true,
		},
		{
			name: "Invalid JSON response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("invalid json{"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer mockServer.Close()

			rs := &ReasoningService{
				apiKey:     "test-key",
				apiURL:     mockServer.URL,
				model:      "test-model",
				httpClient: &http.Client{Timeout: 5 * time.Second},
			}

			ctx := context.Background()
			_, err := rs.callLLM(ctx, "test prompt")
			if (err != nil) != tt.wantErr {
				t.Errorf("callLLM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetPromptOnly tests the GetPromptOnly method
func TestGetPromptOnly(t *testing.T) {
	rs := &ReasoningService{}

	snapshot := &SystemSnapshot{
		Timestamp: time.Now(),
		ServerHealth: ServerHealth{
			CPUUsagePercent:    25.0,
			MemoryUsagePercent: 30.0,
			DiskUsagePercent:   40.0,
		},
		AppStatuses: []AppStatus{},
	}

	prompt, err := rs.GetPromptOnly(snapshot)
	if err != nil {
		t.Fatalf("GetPromptOnly() error = %v", err)
	}

	if prompt == "" {
		t.Error("GetPromptOnly() returned empty prompt")
	}

	// Verify it contains the expected template elements
	if !strings.Contains(prompt, "You are an expert, helpful, and cautious Site Reliability Engineer AI") {
		t.Error("Prompt missing expected template text")
	}
}
