package server

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"ontree-node/internal/database"
	"ontree-node/internal/system"
	"os"
	"path/filepath"
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

		log.Printf("User %s logged in successfully", username)

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

// handleSystemVitals returns system vitals as an HTML partial
func (s *Server) handleSystemVitals(w http.ResponseWriter, r *http.Request) {
	vitals, err := system.GetVitals()
	if err != nil {
		log.Printf("Failed to get system vitals: %v", err)
		http.Error(w, "Failed to get system vitals", http.StatusInternalServerError)
		return
	}

	// Return HTML partial with the vitals data
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprintf(w, `
<div class="vitals-content">
	<div class="vital-item">
		<span class="vital-label">CPU:</span>
		<span class="vital-value">%.1f%%</span>
	</div>
	<div class="vital-item">
		<span class="vital-label">Mem:</span>
		<span class="vital-value">%.1f%%</span>
	</div>
	<div class="vital-item">
		<span class="vital-label">Disk:</span>
		<span class="vital-value">%.1f%%</span>
	</div>
</div>`, vitals.CPUPercent, vitals.MemPercent, vitals.DiskPercent); err != nil {
		log.Printf("Error writing response: %v", err)
	}
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

	// Check for active operations for this app
	// Only consider operations created in the last 5 minutes to avoid showing stale operations
	var activeOperationID string
	db := database.GetDB()
	err = db.QueryRow(`
		SELECT id 
		FROM docker_operations 
		WHERE app_name = ? 
		AND status IN (?, ?)
		AND created_at > datetime('now', '-5 minutes')
		ORDER BY created_at DESC
		LIMIT 1
	`, appName, database.StatusPending, database.StatusInProgress).Scan(&activeOperationID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to check for active operations: %v", err)
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["App"] = app
	data["ComposeContent"] = string(composeContent)
	data["ContainerInfo"] = containerInfo
	data["Messages"] = messages
	data["CSRFToken"] = ""
	data["ActiveOperationID"] = activeOperationID

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
	info["name"] = fmt.Sprintf("ontree-%s", appName)

	return info
}

// handleAppStart handles starting an application container
func (s *Server) handleAppStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "start" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Check if Docker is available
	if s.dockerSvc == nil || s.worker == nil {
		http.Error(w, "Docker service not available", http.StatusServiceUnavailable)
		return
	}

	// Create a background operation
	operationID, err := s.createDockerOperation(database.OpTypeStartContainer, appName, nil)
	if err != nil {
		log.Printf("Failed to create operation for app %s: %v", appName, err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash("Failed to start application: unable to create operation", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Enqueue the operation
	s.worker.EnqueueOperation(operationID)

	// Set flash message with operation ID
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}
	session.AddFlash(fmt.Sprintf("Starting application... <div id=\"operation-status\" hx-get=\"/api/docker/operations/%s\" hx-trigger=\"load\" hx-swap=\"innerHTML\"></div>", operationID), "info")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	// Redirect back to app detail page
	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppStop handles stopping an application container
func (s *Server) handleAppStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "stop" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Stop the container
	if s.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return
	}

	err := s.dockerClient.StopApp(appName)
	if err != nil {
		log.Printf("Failed to stop app %s: %v", appName, err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash(fmt.Sprintf("Failed to stop application: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	} else {
		log.Printf("Successfully stopped app: %s", appName)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash("Application stopped successfully", "success")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	}

	// Redirect back to app detail page
	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppRecreate handles recreating an application container
func (s *Server) handleAppRecreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "recreate" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Check if Docker is available
	if s.dockerSvc == nil || s.worker == nil {
		http.Error(w, "Docker service not available", http.StatusServiceUnavailable)
		return
	}

	// Create a background operation
	operationID, err := s.createDockerOperation(database.OpTypeRecreateContainer, appName, nil)
	if err != nil {
		log.Printf("Failed to create operation for app %s: %v", appName, err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash("Failed to recreate application: unable to create operation", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Enqueue the operation
	s.worker.EnqueueOperation(operationID)

	// Set flash message with operation ID
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}
	session.AddFlash(fmt.Sprintf("Recreating application... <div id=\"operation-status\" hx-get=\"/api/docker/operations/%s\" hx-trigger=\"load\" hx-swap=\"innerHTML\"></div>", operationID), "info")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}

	// Redirect back to app detail page
	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppDelete handles deleting an application container
func (s *Server) handleAppDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "delete" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Delete the container
	if s.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return
	}

	err := s.dockerClient.DeleteAppContainer(appName)
	if err != nil {
		log.Printf("Failed to delete app container %s: %v", appName, err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash(fmt.Sprintf("Failed to delete container: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	} else {
		log.Printf("Successfully deleted app container: %s", appName)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash("Container deleted successfully", "success")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	}

	// Redirect back to app detail page
	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppCheckUpdate checks if a newer version of the Docker image is available
func (s *Server) handleAppCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/apps/")
	path = strings.TrimSuffix(path, "/check-update")
	appName := path

	if appName == "" {
		http.Error(w, "App name required", http.StatusBadRequest)
		return
	}

	// Check for image updates
	if s.dockerSvc == nil {
		s.renderUpdateStatus(w, &updateStatusData{
			Error: "Docker service not available",
		})
		return
	}

	updateStatus, err := s.dockerSvc.CheckImageUpdate(appName)
	if err != nil {
		log.Printf("Failed to check image update for %s: %v", appName, err)
		s.renderUpdateStatus(w, &updateStatusData{
			Error: fmt.Sprintf("Failed to check for updates: %v", err),
		})
		return
	}

	// Render the update status partial
	data := &updateStatusData{
		AppName:         appName,
		UpdateAvailable: updateStatus.UpdateAvailable,
		CurrentImageID:  updateStatus.CurrentImageID,
	}

	if updateStatus.Error != "" {
		data.Error = updateStatus.Error
	}

	s.renderUpdateStatus(w, data)
}

// updateStatusData holds data for the update status template
type updateStatusData struct {
	AppName         string
	UpdateAvailable bool
	CurrentImageID  string
	Error           string
}

// renderUpdateStatus renders the update status partial template
func (s *Server) renderUpdateStatus(w http.ResponseWriter, data *updateStatusData) {
	// Create a simple template for the update status
	tmplStr := `{{if .Error}}<span class="text-danger">{{.Error}}</span>{{else if .UpdateAvailable}}<span class="text-warning">Update available</span> <form method="post" action="/apps/{{.AppName}}/update" class="d-inline"><button type="submit" class="btn btn-sm btn-warning confirm-action" data-action="Update Image" data-confirm-text="Confirm Update?"><i>⬇️</i> Update Now</button></form>{{else}}<span class="text-success">Up to date</span>{{end}}`

	tmpl, err := template.New("updateStatus").Parse(tmplStr)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}

// handleAppUpdate initiates the Docker image update process
func (s *Server) handleAppUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/apps/")
	path = strings.TrimSuffix(path, "/update")
	appName := path

	if appName == "" {
		http.Error(w, "App name required", http.StatusBadRequest)
		return
	}

	// Create a Docker operation for the update
	if s.dockerSvc == nil {
		http.Error(w, "Docker service not available", http.StatusServiceUnavailable)
		return
	}

	// Create the operation in the database
	operationID, err := s.createDockerOperation(database.OpTypeUpdateImage, appName, nil)
	if err != nil {
		log.Printf("Failed to create docker operation: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			// Continue anyway - not critical for most operations
		}
		session.AddFlash("Failed to start update operation", "error")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Queue the operation for background processing
	s.worker.EnqueueOperation(operationID)

	// Redirect back to app detail page (which will show the progress)
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}
	session.AddFlash("Image update started", "success")
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
	}
	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleAppControls returns just the control buttons for an app
func (s *Server) handleAppControls(w http.ResponseWriter, r *http.Request) {
	// Extract app name from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "apps" || parts[3] != "controls" {
		http.NotFound(w, r)
		return
	}

	appName := parts[2]

	// Get app details
	if s.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return
	}

	app, err := s.dockerClient.GetAppDetails(s.config.AppsDir, appName)
	if err != nil {
		http.Error(w, "Failed to get app details", http.StatusInternalServerError)
		return
	}

	// Check for active operations
	var activeOperationID string
	db := database.GetDB()
	err = db.QueryRow(`
		SELECT id 
		FROM docker_operations 
		WHERE app_name = ? 
		AND status IN (?, ?)
		AND created_at > datetime('now', '-5 minutes')
		ORDER BY created_at DESC
		LIMIT 1
	`, appName, database.StatusPending, database.StatusInProgress).Scan(&activeOperationID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking active operations: %v", err)
	}

	// Render just the controls
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if activeOperationID != "" {
		// Show disabled button with spinner
		buttonText := "Processing..."
		if app.Status == "not_created" {
			buttonText = "Creating & Starting..."
		}
		if _, err := fmt.Fprintf(w, `<button type="button" class="btn btn-primary" disabled>
			<span class="spinner-border spinner-border-sm" role="status"></span>
			<span>%s</span>
		</button>`, buttonText); err != nil {
			log.Printf("Error writing response: %v", err)
		}
	} else {
		// Show appropriate control buttons based on status
		if app.Status == "running" {
			if _, err := fmt.Fprintf(w, `<form method="post" action="/apps/%s/stop" class="d-inline">
				<button type="submit" class="btn btn-warning confirm-action" 
						data-action="Stop"
						data-confirm-text="Confirm Stop?">
					<i>⏹️</i> Stop
				</button>
			</form>`, appName); err != nil {
				log.Printf("Error writing response: %v", err)
			}
		} else if app.Status != "not_created" {
			if _, err := fmt.Fprintf(w, `<form method="post" action="/apps/%s/start" class="d-inline">
				<button type="submit" class="btn btn-success">
					<i>▶️</i> Start
				</button>
			</form>`, appName); err != nil {
				log.Printf("Error writing response: %v", err)
			}
		} else {
			if _, err := fmt.Fprintf(w, `<form method="post" action="/apps/%s/start" class="d-inline">
				<button type="submit" class="btn btn-primary">
					<i>🚀</i> Create & Start
				</button>
			</form>`, appName); err != nil {
				log.Printf("Error writing response: %v", err)
			}
		}

		// Add delete and recreate buttons if container exists
		if app.Status != "not_created" {
			if _, err := fmt.Fprintf(w, `
			<form method="post" action="/apps/%s/delete" class="d-inline">
				<button type="submit" class="btn btn-danger confirm-action" 
						data-action="Delete Container"
						data-confirm-text="Confirm Delete?">
					<i>🗑️</i> Delete Container
				</button>
			</form>
			<form method="post" action="/apps/%s/recreate" class="d-inline">
				<button type="submit" class="btn btn-info confirm-action" 
						data-action="Recreate"
						data-confirm-text="Confirm Recreate?">
					<i>🔄</i> Recreate
				</button>
			</form>`, appName, appName); err != nil {
				log.Printf("Error writing response: %v", err)
			}
		}
	}

	// Re-initialize the confirm action buttons
	if _, err := fmt.Fprint(w, `<script>
		// Re-initialize two-step confirmation for dynamically loaded buttons
		document.querySelectorAll('.confirm-action').forEach(button => {
			if (button.dataset.initialized) return;
			button.dataset.initialized = 'true';
			
			let timeout;
			const form = button.closest('form');
			const originalHtml = button.innerHTML;
			const confirmText = button.dataset.confirmText || 'Confirm?';
			const icon = button.querySelector('i')?.textContent || '';
			
			button.addEventListener('click', function(e) {
				e.preventDefault();
				
				if (button.classList.contains('confirming')) {
					form.submit();
				} else {
					button.classList.add('confirming');
					button.innerHTML = icon + ' ' + confirmText + '<span class="cancel-confirm">✗</span>';
					
					const cancelBtn = button.querySelector('.cancel-confirm');
					cancelBtn.addEventListener('click', function(e) {
						e.stopPropagation();
						clearTimeout(timeout);
						button.classList.remove('confirming');
						button.innerHTML = originalHtml;
					});
					
					timeout = setTimeout(() => {
						button.classList.remove('confirming');
						button.innerHTML = originalHtml;
					}, 5000);
				}
			});
		});
	</script>`); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
