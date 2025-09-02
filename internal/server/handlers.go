package server

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"ontree-node/internal/caddy"
	"ontree-node/internal/database"
	"ontree-node/internal/yamlutil"
	"ontree-node/pkg/compose"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// handleSetup handles the initial setup page
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	// Check if setup is already complete
	db := database.GetDB()
	var userCount int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var setupComplete bool
	err = db.QueryRow("SELECT is_setup_complete FROM system_setup WHERE id = 1").Scan(&setupComplete)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if userCount > 0 && setupComplete {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == "POST" {
		// Parse form
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		password2 := r.FormValue("password2")
		nodeName := r.FormValue("node_name")
		nodeDescription := r.FormValue("node_description")

		// Validate
		var errors []string
		if username == "" {
			errors = append(errors, "Username is required")
		}
		if password == "" {
			errors = append(errors, "Password is required")
		}
		if password != password2 {
			errors = append(errors, "Passwords do not match")
		}
		if len(password) < 8 {
			errors = append(errors, "Password must be at least 8 characters long")
		}
		if nodeName == "" {
			nodeName = "OnTree Node"
		}

		if len(errors) == 0 {
			// Create the admin user
			user, err := s.createUser(username, password, "", true, true)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to create user: %v", err))
			} else {
				// Update or create system setup
				if setupComplete {
					_, err = db.Exec(`
						UPDATE system_setup 
						SET is_setup_complete = 1, setup_date = ?, node_name = ?, node_description = ?
						WHERE id = 1
					`, time.Now(), nodeName, nodeDescription)
				} else {
					_, err = db.Exec(`
						INSERT INTO system_setup (id, is_setup_complete, setup_date, node_name, node_description)
						VALUES (1, 1, ?, ?, ?)
					`, time.Now(), nodeName, nodeDescription)
				}

				if err != nil {
					log.Printf("Failed to update system setup: %v", err)
				}

				// Log the user in
				session, err := s.sessionStore.Get(r, "ontree-session")
				if err != nil {
					log.Printf("Failed to get session: %v", err)
					// Continue anyway - not critical
				}
				session.Values["user_id"] = user.ID
				if err := session.Save(r, w); err != nil {
					log.Printf("Failed to save session: %v", err)
				}

				log.Printf("Initial setup completed. Admin user: %s, Node: %s", user.Username, nodeName)

				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}

		// Render with errors
		data := struct {
			User      interface{}
			Errors    []string
			FormData  map[string]string
			CSRFToken string
			Messages  []interface{}
		}{
			User:   nil,
			Errors: errors,
			FormData: map[string]string{
				"username":         username,
				"node_name":        nodeName,
				"node_description": nodeDescription,
			},
			CSRFToken: "",
			Messages:  nil,
		}

		tmpl := s.templates["setup"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// GET request - show form
	data := struct {
		User      interface{}
		Errors    []string
		FormData  map[string]string
		CSRFToken string
		Messages  []interface{}
	}{
		User:   nil,
		Errors: nil,
		FormData: map[string]string{
			"node_name": "OnTree Node",
		},
		CSRFToken: "",
		Messages:  nil,
	}

	tmpl := s.templates["setup"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleLogin handles the login page
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Check if user is already authenticated
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}
	if userID, ok := session.Values["user_id"].(int); ok && userID > 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == "POST" {
		// Parse form
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		// Authenticate user
		user, err := s.authenticateUser(username, password)
		if err != nil {
			// Render with error
			data := struct {
				User      interface{}
				Error     string
				Username  string
				CSRFToken string
				Messages  []interface{}
			}{
				User:      nil,
				Error:     "Invalid username or password",
				Username:  username,
				CSRFToken: "",
				Messages:  nil,
			}

			tmpl := s.templates["login"]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				log.Printf("Error rendering login template: %v", err)
				http.Error(w, "Error rendering template", http.StatusInternalServerError)
			}
			return
		}

		// Set session
		session.Values["user_id"] = user.ID
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}

		log.Printf("User %s logged in successfully with user_id=%d", username, user.ID)

		// Redirect to next URL or dashboard
		next := session.Values["next"]
		if nextURL, ok := next.(string); ok && nextURL != "" {
			delete(session.Values, "next")
			if err := session.Save(r, w); err != nil {
				log.Printf("Failed to save session: %v", err)
			}
			// Add login=success query param for PostHog tracking
			if strings.Contains(nextURL, "?") {
				nextURL += "&login=success"
			} else {
				nextURL += "?login=success"
			}
			http.Redirect(w, r, nextURL, http.StatusFound)
		} else {
			http.Redirect(w, r, "/?login=success", http.StatusFound)
		}
		return
	}

	// GET request - show form
	data := struct {
		User      interface{}
		Error     string
		Username  string
		CSRFToken string
		Messages  []interface{}
	}{
		User:      nil,
		Error:     "",
		Username:  "",
		CSRFToken: "",
		Messages:  nil,
	}

	tmpl := s.templates["login"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Error rendering login template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

// handleLogout handles user logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}

	// Clear session
	session.Values["user_id"] = nil
	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	log.Printf("User logged out")

	http.Redirect(w, r, "/login", http.StatusFound)
}

// handleAppDetail handles the application detail page
func (s *Server) handleAppDetail(w http.ResponseWriter, r *http.Request) {
	// Extract app name from URL path
	path := r.URL.Path
	if !strings.HasPrefix(path, "/apps/") {
		http.NotFound(w, r)
		return
	}

	appName := strings.TrimPrefix(path, "/apps/")
	if appName == "" {
		http.NotFound(w, r)
		return
	}

	// Get user from context
	user := getUserFromContext(r.Context())

	// Get app details
	if s.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return
	}

	app, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get app details: %v", err), http.StatusInternalServerError)
		return
	}

	// Read docker-compose.yml content
	composePath := filepath.Join(app.Path, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath)
	if err != nil {
		log.Printf("Failed to read docker-compose.yml: %v", err)
		composeContent = []byte("Failed to read docker-compose.yml")
	}

	// Get container details if it exists
	var containerInfo map[string]interface{}
	if app.Status != "not_created" && app.Status != "error" {
		containerInfo = s.getContainerInfo(appName)
	}

	// Fetch app status from compose service directly
	var appStatus *AppStatusResponse
	if s.composeSvc != nil {
		// Get container status using compose SDK directly
		ctx := context.Background()
		opts := compose.Options{
			WorkingDir: app.Path,
		}

		containers, err := s.composeSvc.PS(ctx, opts)
		if err != nil {
			log.Printf("Failed to get status for app %s: %v", appName, err)
		} else {
			// Build status response
			appStatus = &AppStatusResponse{
				Success:  true,
				App:      appName,
				Services: []ServiceStatusDetail{},
			}

			// Process containers to get service information
			for _, container := range containers {
				// Extract service name from container name
				// Format: {appName}-{serviceName}-1
				serviceName := ""
				if parts := strings.Split(container.Name, "-"); len(parts) >= 2 {
					serviceName = parts[1]
				}

				service := ServiceStatusDetail{
					Name:   serviceName,
					Image:  container.Image,
					Status: strings.ToLower(container.State),
					State:  container.Status,
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

				appStatus.Services = append(appStatus.Services, service)
			}

			// Determine aggregate status
			runningCount := 0
			stoppedCount := 0
			for _, svc := range appStatus.Services {
				if svc.Status == "running" {
					runningCount++
				} else {
					stoppedCount++
				}
			}

			if runningCount > 0 && stoppedCount > 0 {
				appStatus.Status = "partial"
			} else if runningCount > 0 {
				appStatus.Status = "running"
			} else if stoppedCount > 0 {
				appStatus.Status = "stopped"
			} else {
				appStatus.Status = "not_created"
			}

			// Override app status with multi-service status
			app.Status = appStatus.Status
		}
	}

	// Clear any flash messages from session without displaying them
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}
	session.Flashes("error")
	session.Flashes("success")
	session.Flashes("info")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	// Don't pass messages to the template
	var messages []interface{}

	// Fetch app metadata from docker-compose.yml using yamlutil
	metadata, err := yamlutil.ReadComposeMetadata(app.Path)
	hasMetadata := err == nil && metadata != nil

	// Create a DeployedApp-like structure for template compatibility
	deployedApp := struct {
		ID                string
		Name              string
		Subdomain         string
		HostPort          int
		IsExposed         bool
		TailscaleHostname string
		TailscaleExposed  bool
	}{}
	if hasMetadata {
		deployedApp.Name = appName
		deployedApp.Subdomain = metadata.Subdomain
		deployedApp.HostPort = metadata.HostPort
		deployedApp.IsExposed = metadata.IsExposed
		deployedApp.TailscaleHostname = metadata.TailscaleHostname
		deployedApp.TailscaleExposed = metadata.TailscaleExposed
		// Generate a pseudo-ID for template compatibility
		deployedApp.ID = fmt.Sprintf("app-%s", appName)
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["App"] = app
	data["ComposeContent"] = string(composeContent)
	data["ContainerInfo"] = containerInfo
	data["Messages"] = messages
	data["CSRFToken"] = ""
	data["AppStatus"] = appStatus

	// Add deployed app information if available
	if hasMetadata {
		data["DeployedApp"] = deployedApp
		data["HasDeployedApp"] = true
		data["AppEmoji"] = metadata.Emoji // Pass emoji to template

		// Construct full URLs for display
		var urls []string
		if deployedApp.IsExposed && deployedApp.Subdomain != "" {
			if s.config.PublicBaseDomain != "" {
				urls = append(urls, fmt.Sprintf("https://%s.%s", deployedApp.Subdomain, s.config.PublicBaseDomain))
			}
		}
		data["ExposedURLs"] = urls

		// Add Tailscale info if exposed
		if deployedApp.TailscaleExposed && deployedApp.TailscaleHostname != "" {
			data["TailscaleURL"] = fmt.Sprintf("https://%s", deployedApp.TailscaleHostname)
		}
	} else {
		data["HasDeployedApp"] = false
		data["AppEmoji"] = "" // Empty emoji if no metadata
	}

	// Add domain configuration for UI display
	data["HasDomainsConfigured"] = s.config.PublicBaseDomain != ""
	data["PublicBaseDomain"] = s.config.PublicBaseDomain
	data["TailscaleAuthKey"] = s.config.TailscaleAuthKey != "" // Don't expose the actual key

	// Add Caddy availability check
	data["CaddyAvailable"] = s.caddyClient != nil
	data["PlatformSupportsCaddy"] = runtime.GOOS == "linux"

	// Render template
	tmpl, ok := s.templates["app_detail"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// getContainerInfo retrieves detailed container information
func (s *Server) getContainerInfo(appName string) map[string]interface{} {
	info := make(map[string]interface{})

	// For now, return basic info
	// This will be expanded when we implement container management
	info["name"] = appName

	return info
}

// handleAppComposeEdit shows the docker-compose.yml edit form
func (s *Server) handleAppComposeEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "edit" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]
	user := getUserFromContext(r.Context())

	// Get app details
	if s.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return
	}

	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	// Read docker-compose.yml content
	composePath := filepath.Join(appDetails.Path, "docker-compose.yml")
	content, err := os.ReadFile(composePath)
	if err != nil {
		http.Error(w, "Failed to read compose file", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["App"] = appDetails
	data["Content"] = string(content)

	// Render the template
	tmpl := s.templates["app_compose_edit"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Failed to render edit template: %v", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

// handleAppComposeUpdate handles saving the edited docker-compose.yml
func (s *Server) handleAppComposeUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "edit" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]
	user := getUserFromContext(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	newContent := r.FormValue("content")
	if newContent == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate YAML syntax
	if err := yamlutil.ValidateComposeFile(newContent); err != nil {
		// Show error in edit form
		appDetails, _ := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
		data := s.baseTemplateData(user)
		data["App"] = appDetails
		data["Content"] = newContent
		data["Error"] = fmt.Sprintf("Invalid YAML: %v", err)

		tmpl := s.templates["app_compose_edit"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to render template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Get app details
	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	// Write the new content
	composePath := filepath.Join(appDetails.Path, "docker-compose.yml")
	// Use 0644 for docker-compose.yml files as they need to be readable by docker daemon
	if err := os.WriteFile(composePath, []byte(newContent), 0644); err != nil { // #nosec G306 - compose files need to be world-readable
		log.Printf("Failed to write compose file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Check if container is running
	containerRunning := appDetails.Status == "running"

	// Get session for flash message
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	}

	if containerRunning {
		session.AddFlash("Configuration saved. Please restart the container to apply changes.", "warning")
	} else {
		session.AddFlash("Configuration saved successfully.", "success")
	}

	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	// Redirect to app detail page
	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppExpose handles exposing an application to the internet via Caddy
func (s *Server) handleAppExpose(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "expose" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Check if Caddy is available
	if !s.caddyAvailable || s.caddyClient == nil {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Cannot expose app: Caddy is not available. Please ensure Caddy is installed and running.", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get app details from docker
	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		log.Printf("Failed to get app details: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to expose app: app not found", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		log.Printf("Failed to read compose metadata: %v", err)
		// Initialize with defaults if metadata doesn't exist
		metadata = &yamlutil.OnTreeMetadata{
			Subdomain: "",
			HostPort:  0,
			IsExposed: false,
		}
	}

	// Get subdomain from form
	subdomain := r.FormValue("subdomain")
	if subdomain == "" {
		subdomain = appName // Default to app name
	}

	// Update metadata with subdomain from form
	metadata.Subdomain = subdomain

	// Check if already exposed
	if metadata.IsExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("App is already exposed", "info")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get host port from metadata (should have been set during app creation)
	if metadata.HostPort == 0 {
		// Try to extract from compose file if not set
		compose, err := yamlutil.ReadComposeWithMetadata(filepath.Join(appDetails.Path, "docker-compose.yml"))
		if err == nil {
			// Extract port from first service
			for _, service := range compose.Services {
				if svcMap, ok := service.(map[string]interface{}); ok {
					if ports, ok := svcMap["ports"]; ok {
						if portsList, ok := ports.([]interface{}); ok && len(portsList) > 0 {
							if portStr, ok := portsList[0].(string); ok {
								parts := strings.Split(portStr, ":")
								if len(parts) >= 1 {
									if _, err := fmt.Sscanf(parts[0], "%d", &metadata.HostPort); err != nil {
										log.Printf("Failed to parse port from %s: %v", parts[0], err)
									}
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// Generate app ID for route
	appID := fmt.Sprintf("app-%s", appName)
	log.Printf("[Expose] Exposing app %s with subdomain %s on port %d", appName, metadata.Subdomain, metadata.HostPort)

	// Create route config (only for public domain, Tailscale handled separately)
	routeConfig := caddy.CreateRouteConfig(appID, metadata.Subdomain, metadata.HostPort, s.config.PublicBaseDomain, "")

	// Add route to Caddy
	log.Printf("[Expose] Sending route config to Caddy for app %s", appName)
	err = s.caddyClient.AddOrUpdateRoute(routeConfig)
	if err != nil {
		log.Printf("[Expose] Failed to add route to Caddy: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash(fmt.Sprintf("Failed to expose app: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Update compose file metadata
	metadata.IsExposed = true
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		log.Printf("Failed to update compose metadata: %v", err)
		// Try to rollback Caddy change
		_ = s.caddyClient.DeleteRoute(fmt.Sprintf("route-for-app-%s", appID))
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to expose app: could not update metadata", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	log.Printf("Successfully exposed app %s", appName)
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	}

	publicURL := fmt.Sprintf("https://%s.%s", metadata.Subdomain, s.config.PublicBaseDomain)
	session.AddFlash(fmt.Sprintf("App exposed successfully at: %s", publicURL), "success")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppUnexpose handles removing an application from Caddy
func (s *Server) handleAppUnexpose(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "unexpose" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Get app details from docker
	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		log.Printf("Failed to get app details: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: app not found", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		log.Printf("Failed to read compose metadata: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: could not read metadata", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Check if not exposed
	if !metadata.IsExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("App is not exposed", "info")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Delete route from Caddy if client is available
	if s.caddyClient != nil {
		appID := fmt.Sprintf("app-%s", appName)
		routeID := fmt.Sprintf("route-for-app-%s", appID)
		err = s.caddyClient.DeleteRoute(routeID)
		if err != nil {
			log.Printf("Failed to delete route from Caddy: %v", err)
			// Continue anyway - we'll update the metadata
		}
	}

	// Update compose file metadata
	metadata.IsExposed = false
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		log.Printf("Failed to update compose metadata: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: could not update metadata", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	log.Printf("Successfully unexposed app %s", appName)
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	}
	session.AddFlash("App unexposed successfully", "success")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleSettings handles the settings page display
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())

	// Get current system setup
	var setup database.SystemSetup
	err := s.db.QueryRow(`
		SELECT id, public_base_domain, tailscale_auth_key, tailscale_tags,
		       agent_enabled, agent_check_interval, agent_llm_api_key,
		       agent_llm_api_url, agent_llm_model,
		       uptime_kuma_base_url
		FROM system_setup 
		WHERE id = 1
	`).Scan(&setup.ID, &setup.PublicBaseDomain, &setup.TailscaleAuthKey, &setup.TailscaleTags,
		&setup.AgentEnabled, &setup.AgentCheckInterval, &setup.AgentLLMAPIKey,
		&setup.AgentLLMAPIURL, &setup.AgentLLMModel,
		&setup.UptimeKumaBaseURL)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to get system setup: %v", err)
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	// Get flash messages
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	}

	var messages []interface{}
	if flashes := session.Flashes("success"); len(flashes) > 0 {
		for _, flash := range flashes {
			messages = append(messages, map[string]interface{}{
				"Type": "success",
				"Text": flash,
			})
		}
	}
	if flashes := session.Flashes("error"); len(flashes) > 0 {
		for _, flash := range flashes {
			messages = append(messages, map[string]interface{}{
				"Type": "danger",
				"Text": flash,
			})
		}
	}
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["Messages"] = messages
	data["PublicBaseDomain"] = ""
	data["TailscaleAuthKey"] = ""
	data["TailscaleTags"] = ""
	data["AgentEnabled"] = false
	data["AgentCheckInterval"] = "5m"
	data["AgentLLMAPIKey"] = ""
	data["AgentLLMAPIURL"] = ""
	data["AgentLLMModel"] = ""
	data["UptimeKumaBaseURL"] = ""

	if setup.PublicBaseDomain.Valid {
		data["PublicBaseDomain"] = setup.PublicBaseDomain.String
	}
	if setup.TailscaleAuthKey.Valid {
		data["TailscaleAuthKey"] = setup.TailscaleAuthKey.String
	}
	if setup.TailscaleTags.Valid {
		data["TailscaleTags"] = setup.TailscaleTags.String
	}
	if setup.AgentEnabled.Valid {
		data["AgentEnabled"] = setup.AgentEnabled.Int64 == 1
	}
	if setup.AgentCheckInterval.Valid {
		data["AgentCheckInterval"] = setup.AgentCheckInterval.String
	}
	if setup.AgentLLMAPIKey.Valid {
		data["AgentLLMAPIKey"] = setup.AgentLLMAPIKey.String
	}
	if setup.AgentLLMAPIURL.Valid {
		data["AgentLLMAPIURL"] = setup.AgentLLMAPIURL.String
	}
	if setup.AgentLLMModel.Valid {
		data["AgentLLMModel"] = setup.AgentLLMModel.String
	}
	if setup.UptimeKumaBaseURL.Valid {
		data["UptimeKumaBaseURL"] = setup.UptimeKumaBaseURL.String
	}

	// Also show current values from config (to show if env vars are overriding)
	data["ConfigPublicDomain"] = s.config.PublicBaseDomain
	data["ConfigTailscaleAuthKey"] = s.config.TailscaleAuthKey != ""
	data["ConfigTailscaleTags"] = s.config.TailscaleTags
	data["ConfigAgentEnabled"] = s.config.AgentEnabled
	data["ConfigAgentLLMAPIKey"] = s.config.AgentLLMAPIKey != ""

	// Render template
	tmpl, ok := s.templates["settings"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleSettingsUpdate handles saving settings
func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	publicDomain := strings.TrimSpace(r.FormValue("public_base_domain"))
	tailscaleAuthKey := strings.TrimSpace(r.FormValue("tailscale_auth_key"))
	tailscaleTags := strings.TrimSpace(r.FormValue("tailscale_tags"))
	agentEnabled := r.FormValue("agent_enabled") == "on"
	agentCheckInterval := strings.TrimSpace(r.FormValue("agent_check_interval"))
	agentLLMAPIKey := strings.TrimSpace(r.FormValue("agent_llm_api_key"))
	agentLLMAPIURL := strings.TrimSpace(r.FormValue("agent_llm_api_url"))
	agentLLMModel := strings.TrimSpace(r.FormValue("agent_llm_model"))
	uptimeKumaBaseURL := strings.TrimSpace(r.FormValue("uptime_kuma_base_url"))

	// Convert bool to int for database
	agentEnabledInt := 0
	if agentEnabled {
		agentEnabledInt = 1
	}

	// Ensure system_setup record exists
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO system_setup (id, is_setup_complete) 
		VALUES (1, 1)
	`)
	if err != nil {
		log.Printf("Failed to ensure system_setup exists: %v", err)
	}

	// Update database
	_, err = s.db.Exec(`
		UPDATE system_setup 
		SET public_base_domain = ?, tailscale_auth_key = ?, tailscale_tags = ?,
		    agent_enabled = ?, agent_check_interval = ?, agent_llm_api_key = ?,
		    agent_llm_api_url = ?, agent_llm_model = ?,
		    uptime_kuma_base_url = ?
		WHERE id = 1
	`, publicDomain, tailscaleAuthKey, tailscaleTags, agentEnabledInt, agentCheckInterval,
		agentLLMAPIKey, agentLLMAPIURL, agentLLMModel,
		uptimeKumaBaseURL)

	if err != nil {
		log.Printf("Failed to update settings: %v", err)
		session, sessionErr := s.sessionStore.Get(r, "ontree-session")
		if sessionErr != nil {
			log.Printf("Failed to get session: %v", sessionErr)
		} else {
			session.AddFlash("Failed to save settings", "error")
			if saveErr := session.Save(r, w); saveErr != nil {
				log.Printf("Failed to save session: %v", saveErr)
			}
		}
		http.Redirect(w, r, "/settings", http.StatusFound)
		return
	}

	// Update in-memory config if env vars are not set
	if os.Getenv("PUBLIC_BASE_DOMAIN") == "" {
		s.config.PublicBaseDomain = publicDomain
	}
	if os.Getenv("TAILSCALE_AUTH_KEY") == "" {
		s.config.TailscaleAuthKey = tailscaleAuthKey
	}
	if os.Getenv("TAILSCALE_TAGS") == "" {
		s.config.TailscaleTags = tailscaleTags
	}
	if os.Getenv("AGENT_ENABLED") == "" {
		s.config.AgentEnabled = agentEnabled
	}
	if os.Getenv("AGENT_CHECK_INTERVAL") == "" {
		s.config.AgentCheckInterval = agentCheckInterval
	}
	if os.Getenv("AGENT_LLM_API_KEY") == "" {
		s.config.AgentLLMAPIKey = agentLLMAPIKey
	}
	if os.Getenv("AGENT_LLM_API_URL") == "" {
		s.config.AgentLLMAPIURL = agentLLMAPIURL
	}
	if os.Getenv("AGENT_LLM_MODEL") == "" {
		s.config.AgentLLMModel = agentLLMModel
	}
	if os.Getenv("UPTIME_KUMA_BASE_URL") == "" {
		s.config.UptimeKumaBaseURL = uptimeKumaBaseURL
	}

	// Re-check Caddy health since domains may have changed
	s.checkCaddyHealth()

	// Restart agent if configuration changed
	if err := s.restartAgent(); err != nil {
		log.Printf("Failed to restart agent: %v", err)
	}

	// Success message
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	} else {
		session.AddFlash("Settings saved successfully", "success")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	}

	http.Redirect(w, r, "/settings", http.StatusFound)
}

// handleAppStatusCheck handles checking the status of an exposed application's subdomains
func (s *Server) handleAppStatusCheck(w http.ResponseWriter, r *http.Request) {
	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 || parts[1] != "api" || parts[2] != "apps" || parts[4] != "status" {
		http.NotFound(w, r)
		return
	}

	appName := parts[3]

	// Get app details from docker
	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<div class="alert alert-warning">App not found</div>`))
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<div class="alert alert-warning">Could not read app metadata</div>`))
		return
	}

	if !metadata.IsExposed || metadata.Subdomain == "" {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<div class="alert alert-info">App is not exposed</div>`))
		return
	}

	// Prepare status results
	type StatusResult struct {
		URL        string
		Success    bool
		StatusCode int
		Error      string
	}

	var results []StatusResult

	// Check public domain if configured
	if s.config.PublicBaseDomain != "" {
		url := fmt.Sprintf("https://%s.%s", metadata.Subdomain, s.config.PublicBaseDomain)
		result := StatusResult{URL: url}

		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 5 redirects
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		resp, err := client.Get(url)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Success = true
			result.StatusCode = resp.StatusCode
			_ = resp.Body.Close()
		}

		results = append(results, result)
	}

	// Check Tailscale if exposed (no longer checking subdomain-based URLs)
	if metadata.TailscaleExposed && metadata.TailscaleHostname != "" {
		url := fmt.Sprintf("https://%s", metadata.TailscaleHostname)
		result := StatusResult{URL: url}

		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 5 redirects
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		resp, err := client.Get(url)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Success = true
			result.StatusCode = resp.StatusCode
			_ = resp.Body.Close()
		}

		results = append(results, result)
	}

	// Generate HTML response
	w.Header().Set("Content-Type", "text/html")

	var html strings.Builder
	html.WriteString(`<div class="status-results">`)

	for _, result := range results {
		if result.Success {
			statusClass := "success"
			statusText := "OK"

			if result.StatusCode >= 400 {
				statusClass = "danger"
				statusText = fmt.Sprintf("HTTP %d", result.StatusCode)
			} else if result.StatusCode >= 300 {
				statusClass = "warning"
				statusText = fmt.Sprintf("HTTP %d (Redirect)", result.StatusCode)
			}

			html.WriteString(fmt.Sprintf(`
				<div class="alert alert-%s d-flex justify-content-between align-items-center">
					<div>
						<strong>%s</strong><br>
						<small class="text-muted">Status: %s</small>
					</div>
					<span class="badge bg-%s">%s</span>
				</div>
			`, statusClass, result.URL, statusText, statusClass, statusText))
		} else {
			// Parse error for better display
			errorMsg := result.Error
			if strings.Contains(errorMsg, "no such host") {
				errorMsg = "Could not resolve domain"
			} else if strings.Contains(errorMsg, "connection refused") {
				errorMsg = "Connection refused"
			} else if strings.Contains(errorMsg, "timeout") {
				errorMsg = "Connection timeout"
			} else if strings.Contains(errorMsg, "certificate") {
				errorMsg = "Certificate error"
			}

			html.WriteString(fmt.Sprintf(`
				<div class="alert alert-danger d-flex justify-content-between align-items-center">
					<div>
						<strong>%s</strong><br>
						<small class="text-muted">Error: %s</small>
					</div>
					<span class="badge bg-danger">Failed</span>
				</div>
			`, result.URL, errorMsg))
		}
	}

	html.WriteString(`</div>`)
	_, _ = w.Write([]byte(html.String()))
}

// handleAppContainers returns the running containers for an app
func (s *Server) handleAppContainers(w http.ResponseWriter, r *http.Request) {
	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/apps/")
	appName := strings.TrimSuffix(path, "/containers")

	if appName == "" {
		http.Error(w, "App name required", http.StatusBadRequest)
		return
	}

	// Use the app name as the project name (Docker Compose will use directory name)
	projectName := appName

	// Execute docker ps command with filter for the project
	cmd := fmt.Sprintf(`docker ps --filter "label=com.docker.compose.project=%s" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"`, projectName)

	output, err := s.executeCommand(cmd)
	if err != nil {
		// If no containers found, check for single-service container
		cmd = fmt.Sprintf(`docker ps --filter "name=^%s$" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"`, projectName)
		output, err = s.executeCommand(cmd)
		if err != nil {
			w.Write([]byte(`<div class="text-muted">No running containers found</div>`))
			return
		}
	}

	// Return the output wrapped in a pre tag for proper formatting
	html := fmt.Sprintf(`<pre class="mb-0" style="font-size: 0.875rem;">%s</pre>`, template.HTMLEscapeString(output))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// executeCommand executes a shell command and returns the output
func (s *Server) executeCommand(cmd string) (string, error) {
	// Use bash to execute the command
	execCmd := exec.Command("bash", "-c", cmd)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}
	return string(output), nil
}

// handleAppExposeTailscale handles exposing an application via Tailscale sidecar
func (s *Server) handleAppExposeTailscale(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "expose-tailscale" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Check if Tailscale auth key is configured
	if s.config.TailscaleAuthKey == "" {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Cannot expose app via Tailscale: Auth key not configured in settings", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get app details from docker
	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		log.Printf("Failed to get app details: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to expose app: app not found", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		log.Printf("Failed to read compose metadata: %v", err)
		// Initialize with defaults if metadata doesn't exist
		metadata = &yamlutil.OnTreeMetadata{}
	}

	// Check if already exposed via Tailscale
	if metadata.TailscaleExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("App is already exposed via Tailscale", "info")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get hostname from form or default to app name
	hostname := r.FormValue("hostname")
	if hostname == "" {
		hostname = appName
	}

	log.Printf("[Tailscale Expose] Exposing app %s with hostname %s", appName, hostname)

	// Modify docker-compose.yml to add Tailscale sidecar
	err = yamlutil.ModifyComposeForTailscale(appDetails.Path, appName, hostname, s.config.TailscaleAuthKey)
	if err != nil {
		log.Printf("[Tailscale Expose] Failed to modify compose file: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash(fmt.Sprintf("Failed to expose app via Tailscale: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Update metadata
	metadata.TailscaleHostname = hostname
	metadata.TailscaleExposed = true
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		log.Printf("Failed to update compose metadata: %v", err)
	}

	// Restart containers with new configuration
	log.Printf("[Tailscale Expose] Restarting containers for app %s", appName)
	cmd := fmt.Sprintf("cd %s && docker-compose down && docker-compose up -d", appDetails.Path)
	output, err := s.executeCommand(cmd)
	if err != nil {
		log.Printf("[Tailscale Expose] Failed to restart containers: %v, output: %s", err, output)
		// Try to rollback
		_ = yamlutil.RestoreComposeFromTailscale(appDetails.Path)
		metadata.TailscaleHostname = ""
		metadata.TailscaleExposed = false
		_ = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)

		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to restart containers after adding Tailscale", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Success
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	}

	tailscaleURL := fmt.Sprintf("https://%s", hostname)
	if s.config.TailscaleTags != "" {
		tailscaleURL = fmt.Sprintf("https://%s (with tags: %s)", hostname, s.config.TailscaleTags)
	}

	session.AddFlash(fmt.Sprintf("App exposed via Tailscale at: %s", tailscaleURL), "success")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppUnexposeTailscale handles removing Tailscale sidecar from an application
func (s *Server) handleAppUnexposeTailscale(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "unexpose-tailscale" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Get app details from docker
	appDetails, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		log.Printf("Failed to get app details: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: app not found", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		log.Printf("Failed to read compose metadata: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: could not read metadata", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Check if exposed via Tailscale
	if !metadata.TailscaleExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("App is not exposed via Tailscale", "info")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	log.Printf("[Tailscale Unexpose] Removing Tailscale from app %s", appName)

	// Stop containers first
	cmd := fmt.Sprintf("cd %s && docker-compose down", appDetails.Path)
	output, err := s.executeCommand(cmd)
	if err != nil {
		log.Printf("[Tailscale Unexpose] Warning: Failed to stop containers: %v, output: %s", err, output)
	}

	// Remove Tailscale sidecar from docker-compose.yml
	err = yamlutil.RestoreComposeFromTailscale(appDetails.Path)
	if err != nil {
		log.Printf("[Tailscale Unexpose] Failed to restore compose file: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash(fmt.Sprintf("Failed to remove Tailscale: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Update metadata
	metadata.TailscaleHostname = ""
	metadata.TailscaleExposed = false
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		log.Printf("Failed to update compose metadata: %v", err)
	}

	// Restart containers with original configuration
	log.Printf("[Tailscale Unexpose] Restarting containers for app %s", appName)
	cmd = fmt.Sprintf("cd %s && docker-compose up -d", appDetails.Path)
	output, err = s.executeCommand(cmd)
	if err != nil {
		log.Printf("[Tailscale Unexpose] Failed to restart containers: %v, output: %s", err, output)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		}
		session.AddFlash("Warning: Tailscale removed but failed to restart containers", "warning")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Success
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	}

	session.AddFlash("App removed from Tailscale successfully", "success")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}
