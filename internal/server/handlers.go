package server

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"ontree-node/internal/database"
	"ontree-node/internal/docker"
	"ontree-node/internal/system"
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
				session, _ := s.sessionStore.Get(r, "ontree-session")
				session.Values["user_id"] = user.ID
				session.Save(r, w)

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
		}{
			User:   nil,
			Errors: errors,
			FormData: map[string]string{
				"username":         username,
				"node_name":        nodeName,
				"node_description": nodeDescription,
			},
			CSRFToken: "",
		}

		tmpl := s.templates["setup"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.ExecuteTemplate(w, "base", data)
		return
	}

	// GET request - show form
	data := struct {
		User      interface{}
		Errors    []string
		FormData  map[string]string
		CSRFToken string
	}{
		User:   nil,
		Errors: nil,
		FormData: map[string]string{
			"node_name": "OnTree Node",
		},
		CSRFToken: "",
	}

	tmpl := s.templates["setup"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "base", data)
}

// handleLogin handles the login page
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Check if user is already authenticated
	session, _ := s.sessionStore.Get(r, "ontree-session")
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
			}{
				User:      nil,
				Error:     "Invalid username or password",
				Username:  username,
				CSRFToken: "",
			}

			tmpl := s.templates["login"]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		// Set session
		session.Values["user_id"] = user.ID
		session.Save(r, w)

		log.Printf("User %s logged in successfully", username)

		// Redirect to next URL or dashboard
		next := session.Values["next"]
		if nextURL, ok := next.(string); ok && nextURL != "" {
			delete(session.Values, "next")
			session.Save(r, w)
			http.Redirect(w, r, nextURL, http.StatusFound)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
		return
	}

	// GET request - show form
	data := struct {
		User      interface{}
		Error     string
		Username  string
		CSRFToken string
	}{
		User:      nil,
		Error:     "",
		Username:  "",
		CSRFToken: "",
	}

	tmpl := s.templates["login"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "base", data)
}

// handleLogout handles user logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := s.sessionStore.Get(r, "ontree-session")
	
	// Clear session
	session.Values["user_id"] = nil
	session.Options.MaxAge = -1
	session.Save(r, w)

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
	fmt.Fprintf(w, `
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
</div>`, vitals.CPUPercent, vitals.MemPercent, vitals.DiskPercent)
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
	
	// Prepare template data
	data := struct {
		User           interface{}
		UserInitial    string
		App            *docker.App
		ComposeContent string
		ContainerInfo  map[string]interface{}
		Messages       []interface{}
		CSRFToken      string
	}{
		User:           user,
		UserInitial:    getUserInitial(user.Username),
		App:            app,
		ComposeContent: string(composeContent),
		ContainerInfo:  containerInfo,
		Messages:       nil,
		CSRFToken:      "",
	}
	
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