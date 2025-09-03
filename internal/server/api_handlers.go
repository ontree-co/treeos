package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ontree-node/internal/security"
	"ontree-node/internal/yamlutil"
	"ontree-node/pkg/compose"
)

// appNameRegex validates app names - only lowercase letters, numbers, and hyphens
var appNameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

// CreateAppRequest represents the request body for creating a new app
type CreateAppRequest struct {
	Name        string `json:"name"`
	ComposeYAML string `json:"compose_yaml"`
	EnvContent  string `json:"env_content,omitempty"`
}

// UpdateAppRequest represents the request body for updating an existing app
type UpdateAppRequest struct {
	ComposeYAML string `json:"compose_yaml"`
	EnvContent  string `json:"env_content,omitempty"`
}

// AppStatusResponse represents the response for app status endpoint
type AppStatusResponse struct {
	Success  bool                  `json:"success"`
	App      string                `json:"app"`
	Status   string                `json:"status"`
	Services []ServiceStatusDetail `json:"services"`
	Error    string                `json:"error,omitempty"`
}

// ServiceStatusDetail represents status detail for a single service
type ServiceStatusDetail struct {
	Name   string   `json:"name"`
	Image  string   `json:"image"`
	Status string   `json:"status"`
	State  string   `json:"state,omitempty"`
	Ports  []string `json:"ports,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// handleCreateApp handles POST /api/apps
func (s *Server) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req CreateAppRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate app name
	if req.Name == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	if !appNameRegex.MatchString(req.Name) {
		http.Error(w, "Invalid app name. Only lowercase letters, numbers, and hyphens are allowed", http.StatusBadRequest)
		return
	}

	// Validate YAML content
	if req.ComposeYAML == "" {
		http.Error(w, "Docker Compose YAML is required", http.StatusBadRequest)
		return
	}

	// Validate compose file
	if err := yamlutil.ValidateComposeFile(req.ComposeYAML); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create app directory structure
	appDir := filepath.Join(s.config.AppsDir, req.Name)
	mountDir := filepath.Join(s.config.AppsDir, "mount", req.Name)

	// Check if app already exists
	if _, err := os.Stat(appDir); err == nil {
		http.Error(w, fmt.Sprintf("App '%s' already exists", req.Name), http.StatusConflict)
		return
	}

	// Create directories
	if err := os.MkdirAll(appDir, 0755); err != nil {
		log.Printf("Failed to create app directory: %v", err)
		http.Error(w, "Failed to create app directory", http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll(mountDir, 0755); err != nil {
		log.Printf("Failed to create mount directory: %v", err)
		// Clean up app directory
		os.RemoveAll(appDir)
		http.Error(w, "Failed to create mount directory", http.StatusInternalServerError)
		return
	}

	// Write docker-compose.yml
	composeFile := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(req.ComposeYAML), 0600); err != nil { // #nosec G306 - compose files need to be readable
		log.Printf("Failed to write docker-compose.yml: %v", err)
		// Clean up directories
		os.RemoveAll(appDir)
		os.RemoveAll(mountDir)
		http.Error(w, "Failed to write docker-compose.yml", http.StatusInternalServerError)
		return
	}

	// Write .env file if provided
	if req.EnvContent != "" {
		envFile := filepath.Join(appDir, ".env")
		if err := os.WriteFile(envFile, []byte(req.EnvContent), 0600); err != nil { // #nosec G306 - env files need to be readable
			log.Printf("Failed to write .env file: %v", err)
			// Clean up
			os.RemoveAll(appDir)
			os.RemoveAll(mountDir)
			http.Error(w, "Failed to write .env file", http.StatusInternalServerError)
			return
		}
	}

	// Automatically create and start containers if compose service is available
	if s.composeSvc != nil {
		log.Printf("Starting containers for newly created app: %s at path: %s", req.Name, appDir)

		// Validate security rules first
		validator := security.NewValidator(req.Name)
		if err := validator.ValidateCompose([]byte(req.ComposeYAML)); err != nil {
			log.Printf("Security validation failed for app %s: %v", req.Name, err)
			// Don't fail app creation, just skip container creation
		} else {
			// Start the app using compose SDK
			ctx := context.Background()
			opts := compose.Options{
				WorkingDir: appDir,
			}

			// Check if .env file exists
			envFile := filepath.Join(appDir, ".env")
			if _, err := os.Stat(envFile); err == nil {
				opts.EnvFile = ".env"  // Just the filename, not the full path
			}

			log.Printf("Calling compose.Up with WorkingDir: %s", opts.WorkingDir)

			// Create and start the compose project
			if err := s.composeSvc.Up(ctx, opts); err != nil {
				log.Printf("Failed to start containers for app %s: %v", req.Name, err)
				// Don't fail app creation if containers can't be started
				// User can manually start them later
			} else {
				log.Printf("Successfully started containers for app: %s", req.Name)
			}
		}
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("App '%s' created successfully", req.Name),
		"app": map[string]string{
			"name": req.Name,
			"path": appDir,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleUpdateApp handles PUT /api/apps/{appName}
func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req UpdateAppRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate YAML content
	if req.ComposeYAML == "" {
		http.Error(w, "Docker Compose YAML is required", http.StatusBadRequest)
		return
	}

	// Validate compose file
	if err := yamlutil.ValidateComposeFile(req.ComposeYAML); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Write docker-compose.yml
	composeFile := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(req.ComposeYAML), 0600); err != nil { // #nosec G306 - compose files need to be readable
		log.Printf("Failed to write docker-compose.yml: %v", err)
		http.Error(w, "Failed to write docker-compose.yml", http.StatusInternalServerError)
		return
	}

	// Handle .env file
	envFile := filepath.Join(appDir, ".env")
	if req.EnvContent != "" {
		// Write or update .env file
		if err := os.WriteFile(envFile, []byte(req.EnvContent), 0600); err != nil { // #nosec G306 - env files need to be readable
			log.Printf("Failed to write .env file: %v", err)
			http.Error(w, "Failed to write .env file", http.StatusInternalServerError)
			return
		}
	} else {
		// If env content is empty, remove the .env file if it exists
		if _, err := os.Stat(envFile); err == nil {
			if err := os.Remove(envFile); err != nil {
				log.Printf("Failed to remove .env file: %v", err)
				// Non-critical error, continue
			}
		}
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("App '%s' updated successfully", appName),
		"app": map[string]string{
			"name": appName,
			"path": appDir,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleAPIAppStart handles POST /api/apps/{appName}/start
func (s *Server) handleAPIAppStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/start")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Check if compose service is available
	if s.composeSvc == nil {
		http.Error(w, "Compose service not available", http.StatusServiceUnavailable)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Read docker-compose.yml content
	yamlContent, err := os.ReadFile(composeFile)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		} else {
			log.Printf("Failed to read docker-compose.yml for app %s: %v", appName, err)
			http.Error(w, "Failed to read app configuration", http.StatusInternalServerError)
		}
		return
	}

	// Validate security rules
	validator := security.NewValidator(appName)
	if err := validator.ValidateCompose(yamlContent); err != nil {
		log.Printf("Security validation failed for app %s: %v", appName, err)
		http.Error(w, fmt.Sprintf("Security validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Start the app using compose SDK
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Check if .env file exists
	envFile := filepath.Join(appDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		opts.EnvFile = ".env"  // Just the filename, not the full path
	}

	// Start the compose project
	if err := s.composeSvc.Up(ctx, opts); err != nil {
		log.Printf("Failed to start app %s: %v", appName, err)
		http.Error(w, fmt.Sprintf("Failed to start app: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("App '%s' started successfully", appName),
		"app": map[string]string{
			"name":        appName,
			"projectName": appName,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleAPIAppStop handles POST /api/apps/{appName}/stop
func (s *Server) handleAPIAppStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/stop")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Check if compose service is available
	if s.composeSvc == nil {
		http.Error(w, "Compose service not available", http.StatusServiceUnavailable)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Stop the app using compose SDK (without removing volumes)
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Stop the compose project without removing volumes
	if err := s.composeSvc.Down(ctx, opts, false); err != nil {
		log.Printf("Failed to stop app %s: %v", appName, err)
		http.Error(w, fmt.Sprintf("Failed to stop app: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("App '%s' stopped successfully", appName),
		"app": map[string]string{
			"name":        appName,
			"projectName": appName,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleAPIAppDelete handles DELETE /api/apps/{appName}
func (s *Server) handleAPIAppDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Check if compose service is available
	if s.composeSvc == nil {
		http.Error(w, "Compose service not available", http.StatusServiceUnavailable)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Stop the app using compose SDK with volume removal
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Stop the compose project and remove volumes
	if err := s.composeSvc.Down(ctx, opts, true); err != nil {
		log.Printf("Failed to delete app %s: %v", appName, err)
		http.Error(w, fmt.Sprintf("Failed to delete app: %v", err), http.StatusInternalServerError)
		return
	}

	// Remove the app directory
	if err := os.RemoveAll(appDir); err != nil {
		log.Printf("Failed to remove app directory for %s: %v", appName, err)
		// Continue, as Docker resources are already cleaned up
	}

	// Remove the mount directory
	mountDir := filepath.Join(s.config.AppsDir, "mount", appName)
	if err := os.RemoveAll(mountDir); err != nil {
		log.Printf("Failed to remove mount directory for %s: %v", appName, err)
		// Continue, as this is not critical
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("App '%s' deleted successfully", appName),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleAPIAppStatus handles GET /api/apps/{appName}/status
func (s *Server) handleAPIAppStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/status")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Check if compose service is available
	if s.composeSvc == nil {
		http.Error(w, "Compose service not available", http.StatusServiceUnavailable)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Get container status using compose SDK
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	containers, err := s.composeSvc.PS(ctx, opts)
	if err != nil {
		log.Printf("Failed to get status for app %s: %v", appName, err)
		// Return a response indicating the error
		response := AppStatusResponse{
			Success: false,
			App:     appName,
			Status:  "error",
			Error:   fmt.Sprintf("Failed to get container status: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
		return
	}

	// Process container information into service status
	services := make([]ServiceStatusDetail, 0)
	for _, container := range containers {
		// Extract service name from container name
		// Container names follow the pattern: {appName}-{serviceName}-{index}
		serviceName := extractServiceName(container.Name, appName)

		// Map container state to our status
		status := mapContainerState(container.State)

		service := ServiceStatusDetail{
			Name:   serviceName,
			Image:  container.Image,
			Status: status,
			State:  container.State,
		}

		// Add health status if available
		if container.Health != "" && container.Health != "none" {
			service.State = fmt.Sprintf("%s (health: %s)", container.State, container.Health)
		}

		// Add port information
		if container.Publishers != nil && len(container.Publishers) > 0 {
			// Use a map to track unique port mappings (ignoring IP version)
			portMap := make(map[string]bool)
			for _, pub := range container.Publishers {
				if pub.PublishedPort > 0 && pub.TargetPort > 0 {
					portStr := fmt.Sprintf("%d:%d", pub.PublishedPort, pub.TargetPort)
					portMap[portStr] = true
				}
			}
			// Convert map to sorted slice
			ports := make([]string, 0, len(portMap))
			for port := range portMap {
				ports = append(ports, port)
			}
			service.Ports = ports
		}

		services = append(services, service)
	}

	// Calculate aggregate status
	aggregateStatus := calculateAggregateStatus(services)

	// Return status response
	response := AppStatusResponse{
		Success:  true,
		App:      appName,
		Status:   aggregateStatus,
		Services: services,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// extractServiceName extracts the service name from a container name
func extractServiceName(containerName, appName string) string {
	// Remove leading slash if present
	containerName = strings.TrimPrefix(containerName, "/")

	// Expected format: {appName}-{serviceName}-{index}
	prefix := fmt.Sprintf("%s-", appName)
	if strings.HasPrefix(containerName, prefix) {
		remainder := strings.TrimPrefix(containerName, prefix)
		// Find the last dash to separate service name from index
		lastDash := strings.LastIndex(remainder, "-")
		if lastDash > 0 {
			return remainder[:lastDash]
		}
		return remainder
	}

	// Fallback: return the container name as-is
	return containerName
}

// mapContainerState maps Docker container states to our ServiceStatus
func mapContainerState(state string) string {
	switch strings.ToLower(state) {
	case "running":
		return "running"
	case "created", "restarting", "paused":
		return "stopped"
	case "exited", "dead", "removing":
		return "stopped"
	default:
		return "unknown"
	}
}

// calculateAggregateStatus calculates the overall app status based on service statuses
func calculateAggregateStatus(services []ServiceStatusDetail) string {
	if len(services) == 0 {
		return "stopped"
	}

	runningCount := 0
	totalCount := len(services)

	for _, svc := range services {
		if svc.Status == "running" {
			runningCount++
		}
	}

	// Determine aggregate status
	if runningCount == totalCount {
		return "running"
	}
	if runningCount == 0 {
		return "stopped"
	}
	return "partial"
}

// handleGetApp handles GET /api/apps/{appName}
func (s *Server) handleGetApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Read docker-compose.yml
	composeFile := filepath.Join(appDir, "docker-compose.yml")
	composeContent, err := os.ReadFile(composeFile)
	if err != nil {
		log.Printf("Failed to read docker-compose.yml for app %s: %v", appName, err)
		http.Error(w, "Failed to read app configuration", http.StatusInternalServerError)
		return
	}

	// Read .env file if it exists
	var envContent []byte
	envFile := filepath.Join(appDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		envContent, err = os.ReadFile(envFile)
		if err != nil {
			log.Printf("Failed to read .env file for app %s: %v", appName, err)
			// Non-critical error, continue without env content
		}
	}

	// Return app configuration
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"app": map[string]interface{}{
			"name":         appName,
			"compose_yaml": string(composeContent),
			"env_content":  string(envContent),
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleAPIAppLogs handles GET /api/apps/{appName}/logs
func (s *Server) handleAPIAppLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/logs")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Check if compose service is available
	if s.composeSvc == nil {
		http.Error(w, "Compose service not available", http.StatusServiceUnavailable)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Parse query parameters
	serviceFilter := r.URL.Query().Get("service")
	follow := r.URL.Query().Get("follow") == "true"

	// Set up response headers for streaming
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Set up services filter
	var services []string
	if serviceFilter != "" && serviceFilter != "all" {
		services = []string{serviceFilter}
	}

	// For streaming, disable buffering
	if follow {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Flush headers
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Create context that cancels when client disconnects
	ctx := r.Context()

	// Set up compose options
	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Create log writer that streams to HTTP response
	logWriter := compose.LogWriter{
		Out: w,
		Err: w,
	}

	// Stream logs
	err := s.composeSvc.Logs(ctx, opts, services, follow, logWriter)
	if err != nil {
		// If we haven't written anything yet, we can send an error
		if !follow {
			log.Printf("Failed to get logs for app %s: %v", appName, err)
			fmt.Fprintf(w, "\nError retrieving logs: %v\n", err)
		}
	}
}
