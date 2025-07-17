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

	"gopkg.in/yaml.v3"
	"ontree-node/internal/security"
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

	// Parse YAML to validate it
	var composeData map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.ComposeYAML), &composeData); err != nil {
		http.Error(w, fmt.Sprintf("Invalid YAML: %v", err), http.StatusBadRequest)
		return
	}

	// Check if services section exists
	if _, ok := composeData["services"]; !ok {
		http.Error(w, "Docker Compose YAML must contain a 'services' section", http.StatusBadRequest)
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

	// Parse YAML to validate it
	var composeData map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.ComposeYAML), &composeData); err != nil {
		http.Error(w, fmt.Sprintf("Invalid YAML: %v", err), http.StatusBadRequest)
		return
	}

	// Check if services section exists
	if _, ok := composeData["services"]; !ok {
		http.Error(w, "Docker Compose YAML must contain a 'services' section", http.StatusBadRequest)
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
		ProjectName: fmt.Sprintf("ontree-%s", appName),
		WorkingDir:  appDir,
	}

	// Check if .env file exists
	envFile := filepath.Join(appDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		opts.EnvFile = envFile
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
			"projectName": fmt.Sprintf("ontree-%s", appName),
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
		ProjectName: fmt.Sprintf("ontree-%s", appName),
		WorkingDir:  appDir,
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
			"projectName": fmt.Sprintf("ontree-%s", appName),
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
		ProjectName: fmt.Sprintf("ontree-%s", appName),
		WorkingDir:  appDir,
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
