package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"treeos/internal/ollama"
)

// routeAPIModels handles all /api/models/* routes
func (s *Server) routeAPIModels(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle different model routes
	switch {
	case path == "/api/models" && r.Method == http.MethodGet:
		s.handleAPIModelsGet(w, r)
	case path == "/api/models/events":
		s.handleAPIModelsSSE(w, r)
	case strings.HasSuffix(path, "/pull") && r.Method == http.MethodPost:
		// Extract model name from path
		modelName := strings.TrimPrefix(path, "/api/models/")
		modelName = strings.TrimSuffix(modelName, "/pull")
		s.handleAPIModelPull(w, r, modelName)
	case strings.HasSuffix(path, "/retry") && r.Method == http.MethodPost:
		// Extract model name from path
		modelName := strings.TrimPrefix(path, "/api/models/")
		modelName = strings.TrimSuffix(modelName, "/retry")
		s.handleAPIModelRetry(w, r, modelName)
	default:
		http.NotFound(w, r)
	}
}

// ModelsResponse represents the API response for models list
type ModelsResponse struct {
	Models      []ollama.OllamaModel `json:"models"`
	TotalCount  int                  `json:"total_count"`
	HasOllama   bool                 `json:"has_ollama"`
	LastChecked time.Time            `json:"last_checked"`
}

// handleAPIModelsGet returns the list of all models with their current status
func (s *Server) handleAPIModelsGet(w http.ResponseWriter, r *http.Request) {
	// Get all models from database
	models, err := ollama.GetAllModels(s.db)
	if err != nil {
		log.Printf("Failed to get models: %v", err)
		http.Error(w, "Failed to retrieve models", http.StatusInternalServerError)
		return
	}

	// Check if Ollama container is running
	hasOllama := s.checkOllamaContainer()

	// If Ollama is available, get actually installed models
	if hasOllama {
		installedModels := s.getInstalledModels()
		// Update status for models that are actually installed
		for i := range models {
			if isInstalled(models[i].Name, installedModels) {
				// Only update to completed if not currently downloading
				if models[i].Status != ollama.StatusDownloading {
					models[i].Status = ollama.StatusCompleted
					models[i].Progress = 100
				}
			}
		}
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return HTML for HTMX
		s.renderModelsHTML(w, r, models, hasOllama)
		return
	}

	// Return JSON for API requests
	response := ModelsResponse{
		Models:      models,
		TotalCount:  len(models),
		HasOllama:   hasOllama,
		LastChecked: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleAPIModelPull handles model download requests
func (s *Server) handleAPIModelPull(w http.ResponseWriter, r *http.Request, modelName string) {
	// Check if model exists in our curated list
	model, err := ollama.GetModel(s.db, modelName)
	if err != nil {
		log.Printf("Failed to get model: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if model == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	// Check if already downloading or completed
	if model.Status == ollama.StatusDownloading {
		http.Error(w, "Model is already being downloaded", http.StatusConflict)
		return
	}

	if model.Status == ollama.StatusCompleted {
		http.Error(w, "Model is already downloaded", http.StatusConflict)
		return
	}

	// Create a new download job
	job, err := ollama.CreateDownloadJob(s.db, modelName)
	if err != nil {
		log.Printf("Failed to create download job: %v", err)
		http.Error(w, "Failed to queue download", http.StatusInternalServerError)
		return
	}

	// Add job to worker queue
	if s.ollamaWorker != nil {
		s.ollamaWorker.AddJob(*job)
	} else {
		log.Printf("Warning: Ollama worker not initialized")
		http.Error(w, "Download service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Return 202 Accepted immediately
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Download queued",
		"job_id":  job.ID,
		"model":   modelName,
	})
}

// handleAPIModelRetry handles retry requests for failed downloads
func (s *Server) handleAPIModelRetry(w http.ResponseWriter, r *http.Request, modelName string) {
	// Check if model exists
	model, err := ollama.GetModel(s.db, modelName)
	if err != nil {
		log.Printf("Failed to get model: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if model == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	// Only allow retry for failed models
	if model.Status != ollama.StatusFailed {
		http.Error(w, "Model is not in failed state", http.StatusBadRequest)
		return
	}

	// Clear error state
	err = ollama.ClearModelError(s.db, modelName)
	if err != nil {
		log.Printf("Failed to clear model error: %v", err)
		http.Error(w, "Failed to clear error state", http.StatusInternalServerError)
		return
	}

	// Create a new download job
	job, err := ollama.CreateDownloadJob(s.db, modelName)
	if err != nil {
		log.Printf("Failed to create retry job: %v", err)
		http.Error(w, "Failed to queue retry", http.StatusInternalServerError)
		return
	}

	// Add job to worker queue
	if s.ollamaWorker != nil {
		s.ollamaWorker.AddJob(*job)
	} else {
		log.Printf("Warning: Ollama worker not initialized")
		http.Error(w, "Download service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Return 202 Accepted
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Retry queued",
		"job_id":  job.ID,
		"model":   modelName,
	})
}

// handleAPIModelsSSE handles SSE connections for real-time model updates
func (s *Server) handleAPIModelsSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")

	// Create SSE client
	client := &SSEClient{
		AppID:    "models", // Use "models" as the app ID for model updates
		Messages: make(chan string, 100),
		Close:    make(chan bool),
	}

	// Register client with SSE manager
	s.sseManager.RegisterClient("models", client)
	defer s.sseManager.UnregisterClient("models", client)

	// Create a flusher for immediate sending
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: {\"message\": \"Connected to model updates\"}\n\n")
	flusher.Flush()

	// Handle client disconnect
	notify := r.Context().Done()

	// Start heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Main event loop
	for {
		select {
		case message := <-client.Messages:
			// Send message to client
			fmt.Fprint(w, message)
			flusher.Flush()

		case <-heartbeat.C:
			// Send heartbeat
			fmt.Fprintf(w, "event: heartbeat\ndata: ping\n\n")
			flusher.Flush()

		case <-notify:
			// Client disconnected
			log.Printf("SSE: Client disconnected from models stream")
			return

		case <-client.Close:
			// Server is closing this connection
			return
		}
	}
}

// checkOllamaContainer checks if the Ollama container is running
func (s *Server) checkOllamaContainer() bool {
	// Check for containers with the Ollama service label
	cmd := exec.Command("docker", "ps", "--filter", "label=com.docker.compose.service=ollama", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if we found any containers
	containers := strings.TrimSpace(string(output))
	if containers == "" {
		return false
	}

	// Check if there are multiple Ollama containers running
	containerList := strings.Split(containers, "\n")
	if len(containerList) > 1 {
		log.Printf("WARNING: Multiple Ollama containers found: %v", containerList)
	}

	return true
}

// findOllamaContainer finds which Ollama container is running and returns its name
func (s *Server) findOllamaContainer() string {
	// Check for containers with the Ollama service label
	cmd := exec.Command("docker", "ps", "--filter", "label=com.docker.compose.service=ollama", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Get the list of containers
	containers := strings.TrimSpace(string(output))
	if containers == "" {
		return ""
	}

	// Split into individual container names
	containerList := strings.Split(containers, "\n")

	// If multiple containers, log a warning and use the first one
	if len(containerList) > 1 {
		log.Printf("WARNING: Multiple Ollama containers found (%d), using the first one: %s",
			len(containerList), containerList[0])
		log.Printf("All Ollama containers: %v", containerList)
		// In the future, this should probably return an error
	}

	return containerList[0]
}

// getInstalledModels retrieves the list of actually installed models from Ollama
func (s *Server) getInstalledModels() []string {
	// First, find which Ollama container is running
	containerName := s.findOllamaContainer()
	if containerName == "" {
		log.Printf("No Ollama container found")
		return nil
	}

	cmd := exec.Command("docker", "exec", containerName, "ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to list Ollama models from %s: %v", containerName, err)
		return nil
	}

	var models []string
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		// Skip header line
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		// Extract model name (first field)
		fields := strings.Fields(line)
		if len(fields) > 0 {
			// Keep the full model name including tag
			modelName := fields[0]
			models = append(models, modelName)
			log.Printf("Found installed model: %s", modelName)
		}
	}
	return models
}

// isInstalled checks if a model is in the list of installed models
func isInstalled(modelName string, installedModels []string) bool {
	for _, installed := range installedModels {
		if installed == modelName {
			return true
		}
		// Also check without tag
		if idx := strings.Index(installed, ":"); idx > 0 {
			baseName := installed[:idx]
			if baseName == modelName {
				return true
			}
		}
	}
	return false
}

// renderModelsHTML renders the models list as HTML for HTMX requests
func (s *Server) renderModelsHTML(w http.ResponseWriter, r *http.Request, models []ollama.OllamaModel, hasOllama bool) {
	// Group models by category
	var chatModels, codeModels, visionModels []interface{}

	for _, model := range models {
		// Add status text and color for template
		modelData := map[string]interface{}{
			"Name":         model.Name,
			"DisplayName":  model.DisplayName,
			"SizeEstimate": model.SizeEstimate,
			"Description":  model.Description,
			"Category":     model.Category,
			"Status":       model.Status,
			"Progress":     model.Progress,
			"LastError":    model.LastError,
			"StatusText":   formatStatusText(model.Status),
			"StatusColor":  getStatusColorClass(model.Status),
		}

		switch model.Category {
		case "chat":
			chatModels = append(chatModels, modelData)
		case "code":
			codeModels = append(codeModels, modelData)
		case "vision":
			visionModels = append(visionModels, modelData)
		}
	}

	data := map[string]interface{}{
		"HasOllama":    hasOllama,
		"Models":       models,
		"ChatModels":   chatModels,
		"CodeModels":   codeModels,
		"VisionModels": visionModels,
		"TotalCount":   len(models),
	}

	// Use the pre-loaded template
	tmpl, ok := s.templates["models_list"]
	if !ok {
		log.Printf("Models list template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "models-list-partial", data); err != nil {
		log.Printf("Failed to execute models list template: %v", err)
	}
}

// formatStatusText returns user-friendly status text
func formatStatusText(status string) string {
	switch status {
	case ollama.StatusNotDownloaded:
		return "Not Downloaded"
	case ollama.StatusQueued:
		return "Queued"
	case ollama.StatusDownloading:
		return "Downloading"
	case ollama.StatusCompleted:
		return "Installed"
	case ollama.StatusFailed:
		return "Failed"
	default:
		return status
	}
}

// getStatusColorClass returns Bootstrap color class for status
func getStatusColorClass(status string) string {
	switch status {
	case ollama.StatusNotDownloaded:
		return "secondary"
	case ollama.StatusQueued:
		return "info"
	case ollama.StatusDownloading:
		return "primary"
	case ollama.StatusCompleted:
		return "success"
	case ollama.StatusFailed:
		return "danger"
	default:
		return "secondary"
	}
}

// startOllamaWorker initializes the Ollama worker and starts listening for updates
func (s *Server) startOllamaWorker() {
	// Initialize database models
	err := ollama.InitializeModels(s.db)
	if err != nil {
		log.Printf("Failed to initialize Ollama models: %v", err)
		return
	}

	// Create and start worker
	s.ollamaWorker = ollama.NewWorker(s.db)
	s.ollamaWorker.Start(3) // Start with 3 workers

	// Listen for updates and broadcast via SSE
	go func() {
		updates := s.ollamaWorker.GetUpdatesChannel()
		for update := range updates {
			// Broadcast to all connected SSE clients
			s.sseManager.BroadcastMessage("models", map[string]interface{}{
				"event":     "model-update",
				"model":     update.ModelName,
				"status":    update.Status,
				"progress":  update.Progress,
				"error":     update.Error,
				"timestamp": time.Now().Unix(),
			})
		}
	}()

	log.Println("Ollama worker started")
}
