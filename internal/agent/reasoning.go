package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"
)

// ReasoningService handles LLM interactions for system analysis
type ReasoningService struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
	model      string
}

// LLMConfig contains configuration for the LLM service
type LLMConfig struct {
	APIKey string
	APIURL string
	Model  string
}

// NewReasoningService creates a new ReasoningService instance
func NewReasoningService(config LLMConfig) (*ReasoningService, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Default to OpenAI API if not specified
	if config.APIURL == "" {
		config.APIURL = "https://api.openai.com/v1/chat/completions"
	}

	// Default to GPT-4 if model not specified
	if config.Model == "" {
		config.Model = "gpt-4-turbo-preview"
	}

	return &ReasoningService{
		apiKey: config.APIKey,
		apiURL: config.APIURL,
		model:  config.Model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// promptTemplate is the master prompt template for the LLM
const promptTemplate = `You are an expert, helpful, and cautious Site Reliability Engineer AI for a homeserver. Your task is to analyze the following system snapshot by comparing each app's desired state with its actual state. Identify any deviations or problems. Your output will be used to generate a short status message for a user in a chat interface.

The current time is: {{.CurrentTime}}.

Here is the system data in JSON format:
{{.SystemSnapshotJSON}}

Analyze the data and respond ONLY in the following JSON format. Do not add any explanation before or after the JSON block.

{
  "overall_status": "ALL_OK | WARNING | CRITICAL",
  "summary": "A one-sentence, human-readable summary of the server's state. This will be the main text in the chat message. Make it concise and clear.",
  "analysis": [
    {
      "component": "Component Name (e.g., 'Server Health', 'App: Nextcloud')",
      "status": "OK | WARN | FAIL",
      "finding": "A brief description of the finding for this component. This can be used for an expandable 'details' section in the UI."
    }
  ],
  "recommended_actions": [
    {
      "action_key": "A predefined action key from the allowed list.",
      "parameters": { "key": "value" },
      "justification": "Why this action is recommended."
    }
  ]
}

The ONLY allowed values for 'action_key' are:
- "PERSIST_CHAT_MESSAGE" (parameters: {"app_id": "string", "status": "string", "message": "string"})
  IMPORTANT: For app_id, you MUST use the exact app_id field from the AppStatus in the snapshot JSON (not the app_name!)
- "RESTART_CONTAINER" (parameters: {"container_name": "string"}) - Use the container_name field from the service status in the snapshot
- "NO_ACTION" (parameters: {})

IMPORTANT RULES for RESTART_CONTAINER:
1. You MUST use the exact container_name value from the service's container_name field in the snapshot
2. Do NOT construct container names yourself
3. ONLY recommend RESTART_CONTAINER if the service status is "restarting" or if it recently exited (not if it's been exited for a long time)
4. If a service shows as "exited" but there's no evidence it was recently running, this likely means the container doesn't exist yet - DO NOT try to restart it

A CRITICAL issue exists if a service that SHOULD be running (based on expected_services) is missing or exited. A WARNING exists if logs show errors or restart counts are high. If everything is fine, return 'ALL_OK'. 

CRITICAL RULES FOR PERSIST_CHAT_MESSAGE:
1. Create PERSIST_CHAT_MESSAGE actions ONLY for applications listed in app_statuses
2. ALWAYS use the exact app_id from the AppStatus being analyzed
3. NEVER create messages for "system_health", "server_health", "system", or any other ID not in app_statuses
4. Each AppStatus should result in exactly ONE PERSIST_CHAT_MESSAGE action
5. The server_health data is provided for context only - DO NOT create messages about it

Note: Some apps may be defined but not yet created/started. If all services for an app show as "exited" with no recent activity, report this as "App not yet started" rather than trying to restart non-existent containers.`

// GeneratePrompt creates the full prompt from a SystemSnapshot
func (rs *ReasoningService) GeneratePrompt(snapshot *SystemSnapshot) (string, error) {
	// Convert snapshot to JSON
	snapshotJSON, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal system snapshot: %w", err)
	}

	// Create template
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template: %w", err)
	}

	// Prepare template data
	data := struct {
		CurrentTime        string
		SystemSnapshotJSON string
	}{
		CurrentTime:        snapshot.Timestamp.Format(time.RFC3339),
		SystemSnapshotJSON: string(snapshotJSON),
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	return buf.String(), nil
}

// AnalyzeSnapshot sends the system snapshot to the LLM and returns recommendations
func (rs *ReasoningService) AnalyzeSnapshot(ctx context.Context, snapshot *SystemSnapshot) (*LLMResponse, error) {
	// Generate prompt
	log.Printf("Generating LLM prompt for snapshot with %d apps", len(snapshot.AppStatuses))
	prompt, err := rs.GeneratePrompt(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to generate prompt: %w", err)
	}

	// Call LLM API
	log.Printf("Calling LLM API at %s with model %s", rs.apiURL, rs.model)
	responseText, err := rs.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM: %w", err)
	}
	log.Printf("LLM API response received, parsing...")

	// Parse response
	response, err := rs.parseResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return response, nil
}

// callLLM makes the actual API call to the LLM service
func (rs *ReasoningService) callLLM(ctx context.Context, prompt string) (string, error) {
	// Prepare request body for OpenAI-compatible API
	requestBody := map[string]interface{}{
		"model": rs.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a helpful Site Reliability Engineer AI. Always respond with valid JSON only.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"max_completion_tokens": 2000,
	}

	// Marshal request body
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", rs.apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", rs.apiKey))

	// Make the request
	resp, err := rs.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse OpenAI response format
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// Check for API error
	if apiResponse.Error != nil {
		return "", fmt.Errorf("LLM API error: %s", apiResponse.Error.Message)
	}

	// Extract content from first choice
	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}

	return apiResponse.Choices[0].Message.Content, nil
}

// parseResponse parses the JSON response from the LLM
func (rs *ReasoningService) parseResponse(responseText string) (*LLMResponse, error) {
	// Clean up the response text (remove any potential markdown formatting)
	responseText = strings.TrimSpace(responseText)

	// Remove markdown code block markers if present
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
		responseText = strings.TrimSpace(responseText)
	} else if strings.HasPrefix(responseText, "```") {
		responseText = strings.TrimPrefix(responseText, "```")
		responseText = strings.TrimSuffix(responseText, "```")
		responseText = strings.TrimSpace(responseText)
	}

	// Parse JSON
	var response LLMResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		// If parsing fails, try to extract JSON from the text
		startIdx := strings.Index(responseText, "{")
		endIdx := strings.LastIndex(responseText, "}")

		if startIdx >= 0 && endIdx > startIdx {
			jsonStr := responseText[startIdx : endIdx+1]
			if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
				return nil, fmt.Errorf("failed to parse LLM response JSON: %w (response: %s)", err, responseText)
			}
		} else {
			return nil, fmt.Errorf("failed to parse LLM response JSON: %w (response: %s)", err, responseText)
		}
	}

	// Validate response
	if err := rs.validateResponse(&response); err != nil {
		return nil, fmt.Errorf("invalid LLM response: %w", err)
	}

	return &response, nil
}

// validateResponse validates the LLM response structure
func (rs *ReasoningService) validateResponse(response *LLMResponse) error {
	// Validate overall status
	validStatuses := map[string]bool{
		StatusAllOK:    true,
		StatusWarning:  true,
		StatusCritical: true,
	}
	if !validStatuses[response.OverallStatus] {
		return fmt.Errorf("invalid overall_status: %s", response.OverallStatus)
	}

	// Validate analysis items
	validComponentStatuses := map[string]bool{
		StatusOK:   true,
		StatusWarn: true,
		StatusFail: true,
	}
	for i, item := range response.Analysis {
		if !validComponentStatuses[item.Status] {
			return fmt.Errorf("invalid status for analysis item %d: %s", i, item.Status)
		}
		if item.Component == "" {
			return fmt.Errorf("empty component for analysis item %d", i)
		}
	}

	// Validate recommended actions
	validActionKeys := map[string]bool{
		ActionPersistChatMessage: true,
		ActionRestartContainer:   true,
		ActionNoAction:           true,
	}
	for i, action := range response.RecommendedActions {
		if !validActionKeys[action.ActionKey] {
			return fmt.Errorf("invalid action_key for recommended action %d: %s", i, action.ActionKey)
		}

		// Validate required parameters for specific actions
		switch action.ActionKey {
		case ActionPersistChatMessage:
			if _, ok := action.Parameters["app_id"]; !ok {
				return fmt.Errorf("missing app_id parameter for PERSIST_CHAT_MESSAGE action %d", i)
			}
			if _, ok := action.Parameters["status"]; !ok {
				return fmt.Errorf("missing status parameter for PERSIST_CHAT_MESSAGE action %d", i)
			}
			if _, ok := action.Parameters["message"]; !ok {
				return fmt.Errorf("missing message parameter for PERSIST_CHAT_MESSAGE action %d", i)
			}
		case ActionRestartContainer:
			if _, ok := action.Parameters["container_name"]; !ok {
				return fmt.Errorf("missing container_name parameter for RESTART_CONTAINER action %d", i)
			}
		}
	}

	return nil
}

// GetPromptOnly returns just the prompt without making an API call (useful for testing)
func (rs *ReasoningService) GetPromptOnly(snapshot *SystemSnapshot) (string, error) {
	return rs.GeneratePrompt(snapshot)
}
