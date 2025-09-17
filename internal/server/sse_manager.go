package server

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	AppID    string
	Messages chan string
	Close    chan bool
}

// SSEManager manages Server-Sent Event connections
type SSEManager struct {
	clients map[string]map[*SSEClient]bool // appID -> clients
	mu      sync.RWMutex
}

// NewSSEManager creates a new SSE manager
func NewSSEManager() *SSEManager {
	return &SSEManager{
		clients: make(map[string]map[*SSEClient]bool),
	}
}

// RegisterClient adds a new SSE client for an app
func (m *SSEManager) RegisterClient(appID string, client *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[appID] == nil {
		m.clients[appID] = make(map[*SSEClient]bool)
	}
	m.clients[appID][client] = true
}

// UnregisterClient removes an SSE client
func (m *SSEManager) UnregisterClient(appID string, client *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clients, ok := m.clients[appID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(m.clients, appID)
		}
	}
}

// BroadcastMessage sends a message to all clients for an app
func (m *SSEManager) BroadcastMessage(appID string, messageData interface{}) {
	m.mu.RLock()
	clients := m.clients[appID]
	m.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	// Extract event type if present in messageData
	eventType := "new-message" // Default event type
	if msgMap, ok := messageData.(map[string]interface{}); ok {
		if event, exists := msgMap["event"]; exists {
			if eventStr, isString := event.(string); isString {
				eventType = eventStr
				// Remove the event field from the data since it's now in the SSE event type
				delete(msgMap, "event")
			}
		}
	}

	// Convert message to JSON
	jsonData, err := json.Marshal(messageData)
	if err != nil {
		log.Printf("Failed to marshal SSE message: %v", err)
		return
	}

	// Format as SSE event with the correct event type
	sseMessage := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData))

	// Send to all clients
	for client := range clients {
		select {
		case client.Messages <- sseMessage:
			// Message sent successfully
		case <-time.After(100 * time.Millisecond):
			// Client is not receiving, likely disconnected
			log.Printf("SSE client for app %s is not receiving, will be cleaned up", appID)
		}
	}
}

// SendHeartbeat sends a heartbeat to keep connections alive
func (m *SSEManager) SendHeartbeat(appID string) {
	m.mu.RLock()
	clients := m.clients[appID]
	m.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	heartbeat := "event: heartbeat\ndata: ping\n\n"

	for client := range clients {
		select {
		case client.Messages <- heartbeat:
			// Heartbeat sent
		default:
			// Client buffer is full, skip
		}
	}
}

// SendToAll sends a message to all connected clients across all apps
func (m *SSEManager) SendToAll(eventType string, messageData interface{}) {
	m.mu.RLock()
	allClients := make(map[*SSEClient]bool)
	for _, clientsMap := range m.clients {
		for client := range clientsMap {
			allClients[client] = true
		}
	}
	m.mu.RUnlock()

	if len(allClients) == 0 {
		return
	}

	// Convert message to JSON
	jsonData, err := json.Marshal(messageData)
	if err != nil {
		log.Printf("Failed to marshal SSE message: %v", err)
		return
	}

	// Format as SSE event with the correct event type
	sseMessage := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData))

	// Send to all clients
	for client := range allClients {
		select {
		case client.Messages <- sseMessage:
			// Message sent successfully
		case <-time.After(100 * time.Millisecond):
			// Client is not receiving, likely disconnected
			log.Printf("SSE client is not receiving, will be cleaned up")
		}
	}
}
