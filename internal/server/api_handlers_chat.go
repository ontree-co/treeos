package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ontree-node/internal/database"
)

// ChatMessagesResponse represents the response for chat messages endpoint
type ChatMessagesResponse struct {
	Success  bool                  `json:"success"`
	AppID    string                `json:"app_id"`
	Messages []ChatMessageResponse `json:"messages"`
	Total    int                   `json:"total"`
	Limit    int                   `json:"limit"`
	Offset   int                   `json:"offset"`
	Error    string                `json:"error,omitempty"`
}

// ChatMessageResponse represents a single chat message in the API response
type ChatMessageResponse struct {
	ID            int    `json:"id"`
	Timestamp     string `json:"timestamp"`
	Message       string `json:"message"`
	SenderType    string `json:"sender_type"`
	SenderName    string `json:"sender_name"`
	AgentModel    string `json:"agent_model,omitempty"`
	AgentProvider string `json:"agent_provider,omitempty"`
	StatusLevel   string `json:"status_level,omitempty"`
	Details       string `json:"details,omitempty"`
}

// handleAPIAppChat handles GET and POST /api/apps/{appID}/chat
func (s *Server) handleAPIAppChat(w http.ResponseWriter, r *http.Request) {
	// Route based on method
	switch r.Method {
	case http.MethodGet:
		s.handleAPIAppChatGet(w, r)
	case http.MethodPost:
		s.handleAPIAppChatPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPIAppChatGet handles GET /api/apps/{appID}/chat
func (s *Server) handleAPIAppChatGet(w http.ResponseWriter, r *http.Request) {

	// Extract app name from path
	path := r.URL.Path
	// Remove /api/apps/ prefix and /chat suffix
	appName := strings.TrimPrefix(path, "/api/apps/")
	appName = strings.TrimSuffix(appName, "/chat")

	if appName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatMessagesResponse{
			Success: false,
			Error:   "App name is required",
		})
		return
	}

	// Convert app name to lowercase to match our ID format
	appID := strings.ToLower(appName)

	// Parse query parameters for pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50 // Default limit
	offset := 0 // Default offset

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get messages from database
	messages, err := database.GetChatMessagesForApp(appID, limit, offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatMessagesResponse{
			Success: false,
			AppID:   appID,
			Error:   "Failed to retrieve chat messages",
		})
		return
	}

	// Get total count for pagination
	total, err := database.CountChatMessagesForApp(appID)
	if err != nil {
		// Log error but don't fail the request
		total = len(messages)
	}

	// Convert to response format
	responseMessages := make([]ChatMessageResponse, len(messages))
	for i, msg := range messages {
		resp := ChatMessageResponse{
			ID:         msg.ID,
			Timestamp:  msg.Timestamp.Format("2006-01-02T15:04:05Z"),
			Message:    msg.Message,
			SenderType: msg.SenderType,
			SenderName: msg.SenderName,
		}
		
		// Add optional fields if they have values
		if msg.AgentModel.Valid {
			resp.AgentModel = msg.AgentModel.String
		}
		if msg.AgentProvider.Valid {
			resp.AgentProvider = msg.AgentProvider.String
		}
		if msg.StatusLevel.Valid {
			resp.StatusLevel = msg.StatusLevel.String
		}
		if msg.Details.Valid {
			resp.Details = msg.Details.String
		}
		
		responseMessages[i] = resp
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatMessagesResponse{
		Success:  true,
		AppID:    appID,
		Messages: responseMessages,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	})
}

// handleTestAgentRun handles POST /api/test/agent-run
// This endpoint is for testing purposes only - it triggers an agent check cycle
func (s *Server) handleTestAgentRun(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if agent is enabled
	if s.agentOrchestrator == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Agent is not enabled",
		})
		return
	}

	// Run the agent check in a goroutine to avoid blocking the HTTP response
	go func() {
		ctx := context.Background()
		if err := s.agentOrchestrator.RunCheck(ctx); err != nil {
			// Log the error but don't return it to the client since we're async
			log.Printf("Test agent run failed: %v", err)
		}
	}()

	// Return success immediately (agent runs async)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Agent check triggered successfully",
	})
}

// handleTestAgentConnection handles POST /api/test-agent
// This endpoint tests the LLM API connection with a simple ping message
func (s *Server) handleTestAgentConnection(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		APIKey string `json:"api_key"`
		APIURL string `json:"api_url"`
		Model  string `json:"model"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.APIKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "API key is required",
		})
		return
	}

	// Set defaults if not provided
	if req.APIURL == "" {
		req.APIURL = "https://api.openai.com/v1/chat/completions"
	}
	if req.Model == "" {
		req.Model = "gpt-4-turbo-preview"
	}

	// Test the connection with a simple ping
	testResponse, err := s.testLLMConnection(req.APIKey, req.APIURL, req.Model)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Still return 200, but with error in body
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return success with the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"response": testResponse,
	})
}

// CreateChatMessageRequest represents a request to create a new chat message
type CreateChatMessageRequest struct {
	Message string `json:"message"`
}

// CreateChatMessageResponse represents the response after creating a chat message
type CreateChatMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleAPIAppChatPost handles POST /api/apps/{appID}/chat
func (s *Server) handleAPIAppChatPost(w http.ResponseWriter, r *http.Request) {
	// Extract app name from path
	path := r.URL.Path
	appName := strings.TrimPrefix(path, "/api/apps/")
	appName = strings.TrimSuffix(appName, "/chat")

	if appName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateChatMessageResponse{
			Success: false,
			Error:   "App name is required",
		})
		return
	}

	// Convert app name to lowercase to match our ID format
	appID := strings.ToLower(appName)

	// Parse JSON request body
	var req CreateChatMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateChatMessageResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate message
	if strings.TrimSpace(req.Message) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateChatMessageResponse{
			Success: false,
			Error:   "Message cannot be empty",
		})
		return
	}

	// Create the chat message
	chatMessage := database.ChatMessage{
		AppID:      appID,
		Timestamp:  time.Now(),
		Message:    req.Message,
		SenderType: database.SenderTypeUser,
		SenderName: "User", // TODO: Get actual username from session
	}

	// Store the message in database
	if err := database.CreateChatMessage(chatMessage); err != nil {
		log.Printf("Failed to create user chat message: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateChatMessageResponse{
			Success: false,
			Error:   "Failed to store message",
		})
		return
	}

	// Handle special commands
	go s.handleUserCommand(appID, req.Message)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(CreateChatMessageResponse{
		Success: true,
		Message: "Message sent successfully",
	})
}

// handleUserCommand processes user commands asynchronously
func (s *Server) handleUserCommand(appID string, message string) {
	// Convert message to lowercase for case-insensitive comparison
	lowerMessage := strings.ToLower(strings.TrimSpace(message))

	// Handle specific commands
	switch lowerMessage {
	case "run check":
		// Trigger agent check if available
		if s.agentOrchestrator != nil {
			ctx := context.Background()
			if err := s.agentOrchestrator.RunCheck(ctx); err != nil {
				log.Printf("Failed to run agent check for app %s: %v", appID, err)
			}
		}
	case "show container status", "check for updates", "restart containers":
		// TODO: Implement these commands
		log.Printf("Command '%s' for app %s not yet implemented", message, appID)
	default:
		// No special handling for other messages
		log.Printf("User message for app %s: %s", appID, message)
	}
}
