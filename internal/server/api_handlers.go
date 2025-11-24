package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"treeos/internal/progress"
	"treeos/internal/security"
	"treeos/internal/systemcheck"
	"treeos/internal/yamlutil"
	"treeos/pkg/compose"
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
	Name          string   `json:"name"`
	ContainerName string   `json:"container_name,omitempty"`
	Image         string   `json:"image"`
	Status        string   `json:"status"`
	State         string   `json:"state,omitempty"`
	Ports         []string `json:"ports,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// SystemCheckResponse represents the response from a system check API call.
type SystemCheckResponse struct {
	Success bool                      `json:"success"`
	Checks  []systemcheck.CheckResult `json:"checks"`
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
	defer r.Body.Close() //nolint:errcheck // Cleanup, error not critical

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
		http.Error(w, "Compose YAML is required", http.StatusBadRequest)
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
	if err := os.MkdirAll(appDir, 0755); err != nil { //nolint:gosec // App directory needs group read access
		log.Printf("Failed to create app directory: %v", err)
		http.Error(w, "Failed to create app directory", http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll(mountDir, 0755); err != nil { //nolint:gosec // Mount directory needs group read access
		log.Printf("Failed to create mount directory: %v", err)
		// Clean up app directory
		os.RemoveAll(appDir) //nolint:errcheck,gosec // Best effort cleanup
		http.Error(w, "Failed to create mount directory", http.StatusInternalServerError)
		return
	}

	// Write docker-compose.yml
	composeFile := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(req.ComposeYAML), 0600); err != nil { // #nosec G306 - compose files need to be readable
		log.Printf("Failed to write docker-compose.yml: %v", err)
		// Clean up directories
		os.RemoveAll(appDir)   //nolint:errcheck,gosec // Best effort cleanup
		os.RemoveAll(mountDir) //nolint:errcheck,gosec // Best effort cleanup
		http.Error(w, "Failed to write docker-compose.yml", http.StatusInternalServerError)
		return
	}

	// Write .env file if provided
	if req.EnvContent != "" {
		envFile := filepath.Join(appDir, ".env")
		if err := os.WriteFile(envFile, []byte(req.EnvContent), 0600); err != nil { // #nosec G306 - env files need to be readable
			log.Printf("Failed to write .env file: %v", err)
			// Clean up
			os.RemoveAll(appDir)   //nolint:errcheck,gosec // Best effort cleanup
			os.RemoveAll(mountDir) //nolint:errcheck,gosec // Best effort cleanup
			http.Error(w, "Failed to write .env file", http.StatusInternalServerError)
			return
		}
	}

	// Attempt to start containers if compose service is available
	if composeSvc, composeErr := s.getComposeService(); composeErr != nil {
		if !errors.Is(composeErr, errComposeUnavailable) {
			log.Printf("Compose service unavailable for app %s: %v", req.Name, composeErr)
		}
	} else {
		log.Printf("Starting containers for newly created app: %s at path: %s", req.Name, appDir)

		// Check if security bypass is enabled for this app
		metadata, err := yamlutil.ReadComposeMetadata(appDir)
		if err != nil {
			log.Printf("Failed to read metadata for app %s, assuming security enabled: %v", req.Name, err)
			metadata = &yamlutil.OnTreeMetadata{}
		}

		// Validate security rules unless bypassed
		shouldStart := false
		if !metadata.BypassSecurity {
			validator := security.NewValidator(req.Name)
			if err := validator.ValidateCompose([]byte(req.ComposeYAML)); err != nil {
				log.Printf("Security validation failed for app %s: %v", req.Name, err)
				// Don't fail app creation, just skip container creation
			} else {
				shouldStart = true
			}
		} else {
			log.Printf("SECURITY: Bypassing security validation for app '%s' (user-configured)", req.Name)
			shouldStart = true
		}

		if shouldStart {
			ctx := context.Background()
			opts := compose.Options{WorkingDir: appDir}

			if _, err := os.Stat(filepath.Join(appDir, ".env")); err == nil {
				opts.EnvFile = ".env"
			}

			log.Printf("Calling compose.Up with WorkingDir: %s", opts.WorkingDir)

			if err := composeSvc.Up(ctx, opts); err != nil {
				log.Printf("Failed to start containers for app %s: %v", req.Name, err)
				if isRuntimeUnavailableError(err) {
					s.markComposeUnhealthy()
				}
				// Don't fail app creation if containers can't be started
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
	defer r.Body.Close() //nolint:errcheck // Cleanup, error not critical

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate YAML content
	if req.ComposeYAML == "" {
		http.Error(w, "Compose YAML is required", http.StatusBadRequest)
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

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	composeSvc, err := s.getComposeService()
	if err != nil {
		status := http.StatusServiceUnavailable
		message := "Compose service not available"
		if !errors.Is(err, errComposeUnavailable) {
			message = fmt.Sprintf("Compose service error: %v", err)
		}
		http.Error(w, message, status)
		return
	}

	// Read docker-compose.yml content
	yamlContent, err := os.ReadFile(composeFile) //nolint:gosec // Path from trusted app directory
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		} else {
			log.Printf("Failed to read docker-compose.yml for app %s: %v", appName, err)
			http.Error(w, "Failed to read app configuration", http.StatusInternalServerError)
		}
		return
	}

	// Check if security bypass is enabled for this app
	metadata, err := yamlutil.ReadComposeMetadata(appDir)
	if err != nil {
		log.Printf("Failed to read metadata for app %s, assuming security enabled: %v", appName, err)
		metadata = &yamlutil.OnTreeMetadata{}
	}

	// Validate security rules unless bypassed
	if !metadata.BypassSecurity {
		validator := security.NewValidator(appName)
		if err := validator.ValidateCompose(yamlContent); err != nil {
			log.Printf("Security validation failed for app %s: %v", appName, err)
			http.Error(w, fmt.Sprintf("Security validation failed: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		log.Printf("SECURITY: Bypassing security validation for app '%s' (user-configured)", appName)
	}

	// Initialize progress tracking
	s.progressTracker.StartOperation(appName, progress.OperationPreparing, "Preparing to start containers...")

	// Start the app using compose SDK with progress tracking
	// Use background context with no timeout - user can cancel via UI if needed
	ctx := context.Background()

	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Check if .env file exists
	envFile := filepath.Join(appDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		opts.EnvFile = ".env" // Just the filename, not the full path
	}

	// Create progress parser to handle Docker output
	parser := progress.NewDockerProgressParser(s.progressTracker)

	// Create progress callback function
	progressCallback := func(line string) {
		log.Printf("[Progress] %s: %s", appName, line)
		parser.ParseLine(appName, line)

		// Send SSE update after each progress update
		if progressInfo, exists := s.progressTracker.GetProgress(appName); exists && s.sseManager != nil {
			progressData := map[string]interface{}{
				"type":     "progress",
				"progress": progressInfo,
			}
			s.sseManager.BroadcastMessage("app-progress-"+appName, progressData)
		}
	}

	// Start the compose project with progress tracking
	startChan := make(chan error, 1)
	go func() {
		startChan <- composeSvc.UpWithProgress(ctx, opts, progressCallback)
	}()

	// Wait for either completion or a shorter timeout for the HTTP response
	select {
	case err := <-startChan:
		// Operation completed within time
		if err != nil {
			log.Printf("Failed to start app %s: %v", appName, err)
			s.progressTracker.SetError(appName, err.Error())

			// Send SSE error update
			if progressInfo, exists := s.progressTracker.GetProgress(appName); exists && s.sseManager != nil {
				progressData := map[string]interface{}{
					"type":     "error",
					"progress": progressInfo,
				}
				s.sseManager.BroadcastMessage("app-progress-"+appName, progressData)
			}

			if isRuntimeUnavailableError(err) {
				s.markComposeUnhealthy()
			}
			http.Error(w, fmt.Sprintf("Failed to start app: %v", err), http.StatusInternalServerError)
			return
		}
		// Mark as complete
		s.progressTracker.CompleteOperation(appName, fmt.Sprintf("App '%s' started successfully", appName))

		// Send SSE completion update
		if progressInfo, exists := s.progressTracker.GetProgress(appName); exists && s.sseManager != nil {
			progressData := map[string]interface{}{
				"type":     "complete",
				"progress": progressInfo,
			}
			s.sseManager.BroadcastMessage("app-progress-"+appName, progressData)
		}
	case <-time.After(3 * time.Second):
		// If it takes more than 3 seconds, return immediately with progress status
		// The operation continues in the background
		log.Printf("App %s is starting in background (pulling images)...", appName)

		// Set up background completion handler
		go func() {
			err := <-startChan
			if err != nil {
				log.Printf("Background start failed for app %s: %v", appName, err)
				s.progressTracker.SetError(appName, err.Error())

				// Send SSE error update
				if progressInfo, exists := s.progressTracker.GetProgress(appName); exists && s.sseManager != nil {
					progressData := map[string]interface{}{
						"type":     "error",
						"progress": progressInfo,
					}
					s.sseManager.BroadcastMessage("app-progress-"+appName, progressData)
				}

				if isRuntimeUnavailableError(err) {
					s.markComposeUnhealthy()
				}
			} else {
				log.Printf("Background start completed successfully for app %s", appName)
				s.progressTracker.CompleteOperation(appName, fmt.Sprintf("App '%s' started successfully", appName))

				// Send SSE completion update
				if progressInfo, exists := s.progressTracker.GetProgress(appName); exists && s.sseManager != nil {
					progressData := map[string]interface{}{
						"type":     "complete",
						"progress": progressInfo,
					}
					s.sseManager.BroadcastMessage("app-progress-"+appName, progressData)
				}
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // 202 Accepted for async operation
		response := map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("App '%s' is starting. Check progress at /api/apps/%s/progress", appName, appName),
			"app": map[string]string{
				"name":        appName,
				"projectName": appName,
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
		return
	}

	// Return success response for quick completions
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

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Stop the app using compose SDK (without removing volumes)
	composeSvc, err := s.getComposeService()
	if err != nil {
		status := http.StatusServiceUnavailable
		message := "Compose service not available"
		if !errors.Is(err, errComposeUnavailable) {
			message = fmt.Sprintf("Compose service error: %v", err)
		}
		http.Error(w, message, status)
		return
	}

	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Stop the compose project without removing volumes
	if err := composeSvc.Down(ctx, opts, false); err != nil {
		log.Printf("Failed to stop app %s: %v", appName, err)
		if isRuntimeUnavailableError(err) {
			s.markComposeUnhealthy()
		}
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

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	composeSvc, err := s.getComposeService()
	if err != nil {
		status := http.StatusServiceUnavailable
		message := "Compose service not available"
		if !errors.Is(err, errComposeUnavailable) {
			message = fmt.Sprintf("Compose service error: %v", err)
		}
		http.Error(w, message, status)
		return
	}

	// Stop the app using compose SDK with volume removal
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	// Stop the compose project and remove volumes
	if err := composeSvc.Down(ctx, opts, true); err != nil {
		log.Printf("Failed to delete app %s: %v", appName, err)
		if isRuntimeUnavailableError(err) {
			s.markComposeUnhealthy()
		}
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

// handleAPIAppSecurityBypass handles POST /api/apps/{appName}/security-bypass
func (s *Server) handleAPIAppSecurityBypass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/security-bypass")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var request struct {
		BypassSecurity bool `json:"bypassSecurity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	// Read current metadata
	metadata, err := yamlutil.ReadComposeMetadata(appDir)
	if err != nil {
		log.Printf("Failed to read metadata for app %s: %v", appName, err)
		// Initialize with empty metadata if file doesn't exist
		metadata = &yamlutil.OnTreeMetadata{}
	}

	// Update the bypass security flag
	metadata.BypassSecurity = request.BypassSecurity

	// Write updated metadata back
	if err := yamlutil.UpdateComposeMetadata(appDir, metadata); err != nil {
		log.Printf("Failed to update metadata for app %s: %v", appName, err)
		http.Error(w, "Failed to update security settings", http.StatusInternalServerError)
		return
	}

	// Log the security bypass change for audit purposes
	if request.BypassSecurity {
		log.Printf("SECURITY: Security validation BYPASSED for app '%s'", appName)
	} else {
		log.Printf("SECURITY: Security validation ENABLED for app '%s'", appName)
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":        true,
		"bypassSecurity": request.BypassSecurity,
		"message":        fmt.Sprintf("Security settings updated for app '%s'", appName),
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

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	composeSvc, err := s.getComposeService()
	if err != nil {
		status := http.StatusServiceUnavailable
		message := "Compose service not available"
		if !errors.Is(err, errComposeUnavailable) {
			message = fmt.Sprintf("Compose service error: %v", err)
		}
		http.Error(w, message, status)
		return
	}

	// Get container status using compose SDK
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appDir,
	}

	containers, err := composeSvc.PS(ctx, opts)
	if err != nil {
		log.Printf("Failed to get status for app %s: %v", appName, err)
		if isRuntimeUnavailableError(err) {
			s.markComposeUnhealthy()
		}
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
			Name:          serviceName,
			ContainerName: container.Name,
			Image:         container.Image,
			Status:        status,
			State:         container.State,
		}

		// Add health status if available
		if container.Health != "" && container.Health != "none" {
			service.State = fmt.Sprintf("%s (health: %s)", container.State, container.Health)
		}

		// Add port information
		if len(container.Ports) > 0 {
			portMap := make(map[string]struct{})
			for _, port := range container.Ports {
				if port.HostPort != "" && port.ContainerPort != "" {
					portStr := fmt.Sprintf("%s:%s", port.HostPort, port.ContainerPort)
					portMap[portStr] = struct{}{}
				}
			}
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

// mapContainerState maps container states to our ServiceStatus
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
	composeContent, err := os.ReadFile(composeFile) //nolint:gosec // Path from trusted app directory
	if err != nil {
		log.Printf("Failed to read docker-compose.yml for app %s: %v", appName, err)
		http.Error(w, "Failed to read app configuration", http.StatusInternalServerError)
		return
	}

	// Read .env file if it exists
	var envContent []byte
	envFile := filepath.Join(appDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		envContent, err = os.ReadFile(envFile) //nolint:gosec // Path from trusted app directory
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

	// Check if app exists
	appDir := filepath.Join(s.config.AppsDir, appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
		return
	}

	composeSvc, err := s.getComposeService()
	if err != nil {
		status := http.StatusServiceUnavailable
		message := "Compose service not available"
		if !errors.Is(err, errComposeUnavailable) {
			message = fmt.Sprintf("Compose service error: %v", err)
		}
		http.Error(w, message, status)
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
	err = composeSvc.Logs(ctx, opts, services, follow, logWriter)
	if err != nil {
		if isRuntimeUnavailableError(err) {
			s.markComposeUnhealthy()
		}
		// If we haven't written anything yet, we can send an error
		if !follow {
			log.Printf("Failed to get logs for app %s: %v", appName, err)
			fmt.Fprintf(w, "\nError retrieving logs: %v\n", err) //nolint:errcheck // Best effort logging
		}
	}
}

func (s *Server) handleSystemCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	runner := systemcheck.NewRunner(s.config)
	results := runner.Run(r.Context())

	resp := SystemCheckResponse{
		Success: true,
		Checks:  results,
	}

	for _, check := range results {
		if check.Status != systemcheck.StatusOK {
			resp.Success = false
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode system check response: %v", err)
	}
}

// handleAPIAppProgress handles GET /api/apps/{appName}/progress
func (s *Server) handleAPIAppProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/progress")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Get progress from tracker
	if s.progressTracker == nil {
		http.Error(w, "Progress tracking not available", http.StatusServiceUnavailable)
		return
	}

	// Check if container is already running - if so, clear stale progress data
	appDir := filepath.Join(s.config.AppsDir, appName)
	if composeSvc, err := s.getComposeService(); err == nil {
		opts := compose.Options{WorkingDir: appDir}
		if containers, err := composeSvc.PS(context.Background(), opts); err == nil {
			// If any container is running, clear stale progress
			isRunning := false
			for _, container := range containers {
				if container.State == "running" {
					isRunning = true
					break
				}
			}
			if isRunning {
				s.progressTracker.RemoveOperation(appName)
			}
		}
	}

	progressInfo, exists := s.progressTracker.GetProgress(appName)
	if !exists {
		// No active operation, return default state
		response := map[string]interface{}{
			"app_name":         appName,
			"operation":        "idle",
			"overall_progress": 0,
			"message":          "No active operation",
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode progress response: %v", err)
		}
		return
	}

	// Return current progress
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(progressInfo); err != nil {
		log.Printf("Failed to encode progress response: %v", err)
	}
}

// handleAPIAppProgressSSE handles SSE connection for app progress updates
func (s *Server) handleAPIAppProgressSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	appName := strings.TrimSuffix(path, "/progress/sse")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")

	// Create SSE client with larger buffer for better reliability
	client := &SSEClient{
		AppID:    "app-progress-" + appName,
		Messages: make(chan string, 256), // Increased buffer size
		Close:    make(chan bool, 1),     // Buffered close channel
	}

	// Register client with SSE manager
	if s.sseManager != nil {
		s.sseManager.RegisterClient("app-progress-"+appName, client)
		log.Printf("SSE client registered for app %s progress updates", appName)
		defer func() {
			s.sseManager.UnregisterClient("app-progress-"+appName, client)
			log.Printf("SSE client disconnected for app %s", appName)
		}()
	} else {
		http.Error(w, "SSE not available", http.StatusServiceUnavailable)
		return
	}

	// Create a flusher for immediate sending
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Check if container is already running - if so, don't send stale progress data
	appDir := filepath.Join(s.config.AppsDir, appName)
	if composeSvc, err := s.getComposeService(); err == nil {
		opts := compose.Options{WorkingDir: appDir}
		if containers, err := composeSvc.PS(context.Background(), opts); err == nil {
			// If any container is running, clear stale progress and don't send initial data
			isRunning := false
			for _, container := range containers {
				if container.State == "running" {
					isRunning = true
					break
				}
			}
			if isRunning {
				// Clear any stale progress data
				s.progressTracker.RemoveOperation(appName)
				log.Printf("Cleared stale progress data for running app: %s", appName)
			}
		}
	}

	// Send initial progress state only if operation is active
	if progressInfo, exists := s.progressTracker.GetProgress(appName); exists {
		// Only send if operation is not complete or error
		if progressInfo.Operation != progress.OperationComplete && progressInfo.Operation != progress.OperationError {
			initialData := map[string]interface{}{
				"type":     "progress",
				"progress": progressInfo,
			}
			if jsonData, err := json.Marshal(initialData); err == nil {
				fmt.Fprintf(w, "event: progress\ndata: %s\n\n", string(jsonData)) //nolint:errcheck // SSE stream
				flusher.Flush()
			}
		} else {
			// Clear completed/error operations that are stale
			s.progressTracker.RemoveOperation(appName)
		}
	}

	// Handle client disconnect
	ctx := r.Context()

	// Keep connection alive and send progress updates
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("SSE client context cancelled for app %s", appName)
			return
		case <-client.Close:
			log.Printf("SSE client closed for app %s", appName)
			return
		case message := <-client.Messages:
			if _, err := fmt.Fprint(w, message); err != nil {
				log.Printf("Failed to write SSE message to client for app %s: %v", appName, err)
				return
			}
			flusher.Flush()
		case <-pingTicker.C:
			// Send keepalive with error handling
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				log.Printf("Failed to send keepalive to SSE client for app %s: %v", appName, err)
				return
			}
			flusher.Flush()
		}
	}
}
