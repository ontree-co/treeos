package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"ontree-node/internal/database"
)

// ChatMessagesResponse represents the response for chat messages endpoint
type ChatMessagesResponse struct {
	Success  bool                     `json:"success"`
	AppID    string                   `json:"app_id"`
	Messages []ChatMessageResponse    `json:"messages"`
	Total    int                      `json:"total"`
	Limit    int                      `json:"limit"`
	Offset   int                      `json:"offset"`
	Error    string                   `json:"error,omitempty"`
}

// ChatMessageResponse represents a single chat message in the API response
type ChatMessageResponse struct {
	ID             int    `json:"id"`
	Timestamp      string `json:"timestamp"`
	StatusLevel    string `json:"status_level"`
	MessageSummary string `json:"message_summary"`
	MessageDetails string `json:"message_details,omitempty"`
}

// handleAPIAppChat handles GET /api/apps/{appID}/chat
func (s *Server) handleAPIAppChat(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app ID from path
	path := r.URL.Path
	// Remove /api/apps/ prefix and /chat suffix
	appID := strings.TrimPrefix(path, "/api/apps/")
	appID = strings.TrimSuffix(appID, "/chat")

	if appID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatMessagesResponse{
			Success: false,
			Error:   "App ID is required",
		})
		return
	}

	// Parse query parameters for pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50 // Default limit
	offset := 0  // Default offset

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
		responseMessages[i] = ChatMessageResponse{
			ID:             msg.ID,
			Timestamp:      msg.Timestamp.Format("2006-01-02T15:04:05Z"),
			StatusLevel:    msg.StatusLevel,
			MessageSummary: msg.MessageSummary,
		}
		if msg.MessageDetails.Valid {
			responseMessages[i].MessageDetails = msg.MessageDetails.String
		}
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