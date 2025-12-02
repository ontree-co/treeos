package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"github.com/ontree-co/treeos/internal/logging"

	"github.com/ontree-co/treeos/internal/caddy"
	"github.com/ontree-co/treeos/internal/database"
	"github.com/ontree-co/treeos/internal/ollama"
	containerruntime "github.com/ontree-co/treeos/internal/runtime"
	"github.com/ontree-co/treeos/internal/yamlutil"
	"github.com/ontree-co/treeos/pkg/compose"

	"gopkg.in/yaml.v3"
)

type appDetailView struct {
	Name           string
	Emoji          string
	Status         string
	StatusLabel    string
	StatusClass    string
	Services       []serviceView
	ServiceOptions []string
	HasServices    bool
	TailscaleDNS   string
	ComposeContent string
	EnvContent     string
	AppYmlContent  string
	ComposePath    string
	EnvPath        string
	AppYmlPath     string
	AppPath        string
	Metadata       metadataView
	PublicAccess   publicAccessView
	Tailscale      tailscaleView
	Security       securityView
	Actions        actionsView
	Warnings       []string
}

type serviceView struct {
	Name          string
	ContainerName string
	Image         string
	Status        string
	StatusLabel   string
	StatusClass   string
	State         string
	Ports         []string
}

func (s *Server) getAppDetailsForRequest(w http.ResponseWriter, r *http.Request, appName string) (*containerruntime.App, bool) {
	app, err := s.getAppDetails(appName)
	if err != nil {
		if errors.Is(err, errRuntimeUnavailable) {
			http.Error(w, "Container runtime not available", http.StatusServiceUnavailable)
			return nil, false
		}
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return nil, false
		}
		http.Error(w, fmt.Sprintf("Failed to get app details: %v", err), http.StatusInternalServerError)
		return nil, false
	}
	return app, true
}

type metadataView struct {
	HasMetadata       bool
	Subdomain         string
	HostPort          int
	IsExposed         bool
	PublicURL         string
	TailscaleExposed  bool
	TailscaleHostname string
	TailscaleURL      string
}

type publicAccessView struct {
	FormEnabled bool
	Exposed     bool
	Subdomain   string
	PublicURL   string
	BaseDomain  string
	Alert       *alertView
}

type tailscaleView struct {
	FormEnabled bool
	Exposed     bool
	Hostname    string
	URL         string
	Alert       *alertView
}

type securityView struct {
	BypassEnabled bool
}

type actionsView struct {
	CanStart bool
	CanStop  bool
}

type alertView struct {
	Type    string
	Message string
}

func capitalizeFirst(s string) string {
	if s == "" {
		return "Unknown"
	}
	if len(s) == 1 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

func statusBadgeClass(status string) string {
	switch status {
	case "running":
		return "bg-success"
	case "partial":
		return "bg-warning"
	case "stopped", "exited", "not_created":
		return "bg-secondary"
	case "error":
		return "bg-danger"
	default:
		return "bg-secondary"
	}
}

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
		logging.Debugf("Setup POST request received")

		// Parse form
		err := r.ParseForm()
		if err != nil {
			logging.Debugf("Failed to parse form: %v", err)
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		password2 := r.FormValue("password2")
		nodeName := r.FormValue("node_name")
		nodeIcon := r.FormValue("node_icon")

		logging.Debugf("Form values - username: %s, nodeName: %s, nodeIcon: %s, password length: %d",
			username, nodeName, nodeIcon, len(password))

		// Default icon if none selected - use first icon to match frontend
		if nodeIcon == "" {
			nodeIcon = "tree0.png"
		}

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

		logging.Debugf("Validation complete. Errors: %v", errors)

		if len(errors) == 0 {
			logging.Debugf("No validation errors, proceeding with session storage")
			// Store setup data in session and redirect to system check
			session, err := s.sessionStore.Get(r, "ontree-session")
			if err != nil {
				logging.Errorf("Failed to get session: %v", err)
				http.Error(w, "Session error", http.StatusInternalServerError)
				return
			}

			// Store setup data in session as individual values
			session.Values["setup_username"] = username
			session.Values["setup_password"] = password
			session.Values["setup_node_name"] = nodeName
			session.Values["setup_node_icon"] = nodeIcon

			if err := session.Save(r, w); err != nil {
				logging.Debugf("Failed to save session with setup data: %v", err)
				http.Error(w, "Session error", http.StatusInternalServerError)
				return
			}

			logging.Debugf("Setup data stored in session successfully, redirecting to system check")
			http.Redirect(w, r, "/systemcheck", http.StatusFound)
			return
		}
		logging.Debugf("Validation failed, re-rendering form with errors")

		// Render with errors
		data := s.baseTemplateData(nil) // nil for user since not logged in
		data["Errors"] = errors
		data["FormData"] = map[string]string{
			"username":  username,
			"node_name": nodeName,
			"node_icon": nodeIcon,
		}

		tmpl := s.templates["setup"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			logging.Errorf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// GET request - show form
	data := s.baseTemplateData(nil) // nil for user since not logged in
	data["Errors"] = nil
	data["FormData"] = map[string]string{
		"node_name": "OnTree Node",
	}

	tmpl := s.templates["setup"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		logging.Errorf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleSetupSystemCheck handles the system check page during setup
func (s *Server) handleSetupSystemCheck(w http.ResponseWriter, r *http.Request) {
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

	// Get session to retrieve setup data
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	if r.Method == "POST" {
		// Get setup data from session
		username, usernameOk := session.Values["setup_username"].(string)
		password, passwordOk := session.Values["setup_password"].(string)
		nodeName, _ := session.Values["setup_node_name"].(string) //nolint:errcheck
		nodeIcon, _ := session.Values["setup_node_icon"].(string) //nolint:errcheck

		if !usernameOk || !passwordOk || username == "" || password == "" {
			logging.Infof("Incomplete setup data in session, redirecting to setup")
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}

		// Check which button was pressed
		action := r.FormValue("action")
		if action != "complete" && action != "continue" {
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}

		// Default icon if none selected
		if nodeIcon == "" {
			nodeIcon = "tree0.png"
		}
		if nodeName == "" {
			nodeName = "OnTree Node"
		}

		// Complete the setup process
		user, err := s.createUser(username, password, "", true, true)
		if err != nil {
			logging.Errorf("Failed to create user during system check: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Update or create system setup
		if setupComplete {
			_, err = db.Exec(`
				UPDATE system_setup
				SET is_setup_complete = 1, setup_date = ?, node_name = ?, node_icon = ?
				WHERE id = 1
			`, time.Now(), nodeName, nodeIcon)
		} else {
			_, err = db.Exec(`
				INSERT INTO system_setup (id, is_setup_complete, setup_date, node_name, node_icon)
				VALUES (1, 1, ?, ?, ?)
			`, time.Now(), nodeName, nodeIcon)
		}

		if err != nil {
			logging.Errorf("Failed to update system setup: %v", err)
		}

		// Log the user in
		session.Values["user_id"] = user.ID
		// Clear setup data from session
		delete(session.Values, "setup_username")
		delete(session.Values, "setup_password")
		delete(session.Values, "setup_node_name")
		delete(session.Values, "setup_node_icon")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}

		logging.Infof("Setup completed via system check. Admin user: %s, Node: %s, Action: %s", user.Username, nodeName, action)

		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// GET request - show system check page
	data := s.baseTemplateData(nil) // nil for user since not logged in
	data["SystemCheckAutoRun"] = true
	data["SystemCheckVisible"] = true
	data["SystemCheckPanelID"] = "system-check"

	tmpl := s.templates["systemcheck"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		logging.Errorf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleLogin handles the login page
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Check if user is already authenticated
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
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
			data := s.baseTemplateData(nil) // nil for user since not logged in
			data["Error"] = "Invalid username or password"
			data["Username"] = username

			tmpl := s.templates["login"]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				logging.Errorf("Error rendering login template: %v", err)
				http.Error(w, "Error rendering template", http.StatusInternalServerError)
			}
			return
		}

		// Set session
		session.Values["user_id"] = user.ID
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}

		logging.Infof("User %s logged in successfully with user_id=%d", username, user.ID)

		// Redirect to next URL or dashboard
		next := session.Values["next"]
		if nextURL, ok := next.(string); ok && nextURL != "" {
			// Clear the next value
			delete(session.Values, "next")
			if err := session.Save(r, w); err != nil {
				logging.Errorf("Failed to save session: %v", err)
			}

			// Skip deprecated/old routes and go to dashboard instead
			if strings.HasPrefix(nextURL, "/monitoring") {
				http.Redirect(w, r, "/?login=success", http.StatusFound)
				return
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
	data := s.baseTemplateData(nil) // nil for user since not logged in
	data["Error"] = ""
	data["Username"] = ""

	tmpl := s.templates["login"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		logging.Errorf("Error rendering login template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

// handleLogout handles user logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}

	// Clear session
	session.Values["user_id"] = nil
	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
	}

	logging.Infof("User logged out")

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

	app, ok := s.getAppDetailsForRequest(w, r, appName)
	if !ok {
		return
	}

	warnings := make([]string, 0)

	// Read docker-compose.yml content
	composePath := filepath.Join(app.Path, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath) //nolint:gosec // Path from trusted app directory
	if err != nil {
		logging.Errorf("Failed to read docker-compose.yml: %v", err)
		composeContent = []byte("Failed to read docker-compose.yml")
		warnings = append(warnings, "Unable to read docker-compose.yml; displaying placeholder content.")
	}

	// Read .env content
	envPath := filepath.Join(app.Path, ".env")
	envContent, err := os.ReadFile(envPath) //nolint:gosec // Path from trusted app directory
	if err != nil {
		logging.Errorf("Failed to read .env: %v", err)
		envContent = []byte("# No .env file found")
		warnings = append(warnings, ".env file not found.")
	}

	// Read app.yml content
	appYmlPath := filepath.Join(app.Path, "app.yml")
	appYmlContent, err := os.ReadFile(appYmlPath) //nolint:gosec // Path from trusted app directory
	if err != nil {
		logging.Errorf("Failed to read app.yml: %v", err)
		appYmlContent = []byte("# No app.yml file found")
		warnings = append(warnings, "app.yml file not found.")
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
			logging.Errorf("Failed to get status for app %s: %v", appName, err)
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
					Name:          serviceName,
					ContainerName: container.Name,
					Image:         container.Image,
					Status:        strings.ToLower(container.State),
					State:         container.Status,
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
		logging.Errorf("Failed to get session: %v", err)
		// Continue anyway - not critical for most operations
	}
	session.Flashes("error")
	session.Flashes("success")
	session.Flashes("info")
	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
	}

	// Don't pass messages to the template
	var messages []interface{}

	// Fetch app metadata from container runtime-compose.yml using yamlutil
	metadata, err := yamlutil.ReadComposeMetadata(app.Path)
	hasMetadata := err == nil && metadata != nil
	if err != nil {
		logging.Errorf("Failed to read compose metadata for app %s: %v", appName, err)
		warnings = append(warnings, "Unable to read x-ontree metadata; using defaults.")
	}

	view := appDetailView{
		Name:           app.Name,
		Emoji:          app.Emoji,
		Status:         app.Status,
		StatusLabel:    capitalizeFirst(app.Status),
		StatusClass:    statusBadgeClass(app.Status),
		ComposeContent: string(composeContent),
		EnvContent:     string(envContent),
		AppYmlContent:  string(appYmlContent),
		ComposePath:    composePath,
		EnvPath:        envPath,
		AppYmlPath:     appYmlPath,
		AppPath:        app.Path,
		TailscaleDNS:   strings.TrimSuffix(getTailscaleDNS(), "."),
		Security:       securityView{BypassEnabled: app.BypassSecurity},
	}

	if view.Emoji == "" && hasMetadata && metadata != nil && metadata.Emoji != "" {
		view.Emoji = metadata.Emoji
	}

	// Populate service information
	if appStatus != nil {
		serviceOptions := make([]string, 0, len(appStatus.Services))
		for _, svc := range appStatus.Services {
			service := serviceView{
				Name:          svc.Name,
				ContainerName: svc.ContainerName,
				Image:         svc.Image,
				Status:        svc.Status,
				StatusLabel:   capitalizeFirst(svc.Status),
				StatusClass:   statusBadgeClass(svc.Status),
				State:         svc.State,
				Ports:         svc.Ports,
			}
			view.Services = append(view.Services, service)
			if svc.Name != "" {
				serviceOptions = append(serviceOptions, svc.Name)
			}
		}
		view.ServiceOptions = serviceOptions
		view.HasServices = len(view.Services) > 0
	}

	// Determine available actions
	actions := actionsView{}
	switch app.Status {
	case "running", "partial":
		actions.CanStop = true
	default:
		actions.CanStart = true
	}
	view.Actions = actions

	// Metadata details
	metadataSummary := metadataView{}
	if hasMetadata && metadata != nil {
		metadataSummary.HasMetadata = true
		metadataSummary.Subdomain = metadata.Subdomain
		metadataSummary.HostPort = metadata.HostPort
		metadataSummary.IsExposed = metadata.IsExposed
		metadataSummary.TailscaleExposed = metadata.TailscaleExposed
		metadataSummary.TailscaleHostname = metadata.TailscaleHostname
		if metadata.TailscaleExposed && metadata.TailscaleHostname != "" {
			metadataSummary.TailscaleURL = fmt.Sprintf("https://%s", metadata.TailscaleHostname)
		}
		if metadata.IsExposed && metadata.Subdomain != "" && s.config.PublicBaseDomain != "" {
			metadataSummary.PublicURL = fmt.Sprintf("https://%s.%s", metadata.Subdomain, s.config.PublicBaseDomain)
		}
	}
	view.Metadata = metadataSummary

	// Public access state
	defaultSubdomain := metadataSummary.Subdomain
	if defaultSubdomain == "" {
		defaultSubdomain = strings.ToLower(app.Name)
	}
	publicAccess := publicAccessView{
		FormEnabled: runtime.GOOS == "linux" && s.caddyClient != nil && s.config.PublicBaseDomain != "",
		Exposed:     metadataSummary.IsExposed,
		Subdomain:   defaultSubdomain,
		PublicURL:   metadataSummary.PublicURL,
		BaseDomain:  s.config.PublicBaseDomain,
	}
	if !publicAccess.FormEnabled {
		var alertMsg, alertType string
		switch {
		case runtime.GOOS != "linux":
			alertMsg = "Domain exposure is only available on Linux servers."
			alertType = "info"
		case s.caddyClient == nil:
			alertMsg = "Caddy service not available. Configure Caddy to enable public exposure."
			alertType = "warning"
		case s.config.PublicBaseDomain == "":
			alertMsg = "No public base domain configured. Add one in Settings to expose apps."
			alertType = "info"
		}
		if alertMsg != "" {
			publicAccess.Alert = &alertView{Type: alertType, Message: alertMsg}
		}
	}
	view.PublicAccess = publicAccess

	// Tailscale state
	tailscale := tailscaleView{
		FormEnabled: s.config.TailscaleAuthKey != "",
		Exposed:     metadataSummary.TailscaleExposed,
		Hostname:    metadataSummary.TailscaleHostname,
		URL:         metadataSummary.TailscaleURL,
	}
	if !tailscale.FormEnabled {
		tailscale.Alert = &alertView{Type: "info", Message: "Tailscale not configured. Add an auth key in Settings to enable this feature."}
	}
	view.Tailscale = tailscale

	// Attach warnings collected during processing
	view.Warnings = warnings

	// Prepare template data
	data := s.baseTemplateData(user)
	data["View"] = view
	data["Messages"] = messages
	data["CSRFToken"] = ""

	// Render template
	tmpl, ok := s.templates["app_detail"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		// Log the full error for debugging
		logging.Errorf("Error rendering app_detail template for app %s: %v", appName, err)
		// Also check if it's a network error (broken pipe) and log differently
		if !strings.Contains(err.Error(), "broken pipe") && !strings.Contains(err.Error(), "connection reset") {
			logging.Errorf("Template execution error (not network): %v", err)
		}
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
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

	appDetails, ok := s.getAppDetailsForRequest(w, r, appName)
	if !ok {
		return
	}

	// Read docker-compose.yml content
	composePath := filepath.Join(appDetails.Path, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath) //nolint:gosec // Path from trusted app directory
	if err != nil {
		http.Error(w, "Failed to read compose file", http.StatusInternalServerError)
		return
	}

	// Read .env content
	envPath := filepath.Join(appDetails.Path, ".env")
	envContent, err := os.ReadFile(envPath) //nolint:gosec // Path from trusted app directory
	if err != nil {
		// .env file might not exist, that's ok
		envContent = []byte("")
	}

	// Read app.yml content
	appYmlPath := filepath.Join(appDetails.Path, "app.yml")
	appYmlContent, err := os.ReadFile(appYmlPath) //nolint:gosec // Path from trusted app directory
	if err != nil {
		// app.yml file might not exist, that's ok
		appYmlContent = []byte("")
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["App"] = appDetails
	data["Content"] = string(composeContent) // Keep for backward compatibility
	data["ComposeContent"] = string(composeContent)
	data["EnvContent"] = string(envContent)
	data["AppYmlContent"] = string(appYmlContent)

	// Render the template
	tmpl := s.templates["app_compose_edit"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		logging.Errorf("Failed to render edit template: %v", err)
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

	// Get all three file contents from form
	composeContent := r.FormValue("compose_content")
	envContent := r.FormValue("env_content")
	appYmlContent := r.FormValue("app_yml_content")

	// For backward compatibility, also check "content" field
	if composeContent == "" {
		composeContent = r.FormValue("content")
	}

	if composeContent == "" {
		http.Error(w, "Docker compose content cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate docker-compose YAML syntax
	if err := yamlutil.ValidateComposeFile(composeContent); err != nil {
		// Show error in edit form
		appDetails, detailErr := s.getAppDetails(appName)
		if detailErr != nil {
			logging.Errorf("Failed to load app details for %s during compose validation: %v", appName, detailErr)
			appDetails = &containerruntime.App{
				Name: appName,
				Path: filepath.Join(s.config.AppsDir, appName),
			}
		}
		data := s.baseTemplateData(user)
		data["App"] = appDetails
		data["ComposeContent"] = composeContent
		data["EnvContent"] = envContent
		data["AppYmlContent"] = appYmlContent
		data["Error"] = fmt.Sprintf("Invalid docker-compose.yml: %v", err)

		tmpl := s.templates["app_compose_edit"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			logging.Errorf("Failed to render template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Validate app.yml YAML syntax if provided
	if appYmlContent != "" {
		var appYml map[string]interface{}
		if err := yaml.Unmarshal([]byte(appYmlContent), &appYml); err != nil {
			// Show error in edit form
			appDetails, detailErr := s.getAppDetails(appName)
			if detailErr != nil {
				logging.Errorf("Failed to load app details for %s during app.yml validation: %v", appName, detailErr)
				appDetails = &containerruntime.App{
					Name: appName,
					Path: filepath.Join(s.config.AppsDir, appName),
				}
			}
			data := s.baseTemplateData(user)
			data["App"] = appDetails
			data["ComposeContent"] = composeContent
			data["EnvContent"] = envContent
			data["AppYmlContent"] = appYmlContent
			data["Error"] = fmt.Sprintf("Invalid app.yml: %v", err)

			tmpl := s.templates["app_compose_edit"]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
				logging.Errorf("Failed to render template: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}
	}

	// Get app details
	appDetails, ok := s.getAppDetailsForRequest(w, r, appName)
	if !ok {
		return
	}

	// Write docker-compose.yml
	composePath := filepath.Join(appDetails.Path, "docker-compose.yml")
	// Use 0644 for docker-compose.yml files as they need to be readable by docker daemon
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil { //nolint:gosec // Compose files need to be world-readable
		logging.Errorf("Failed to write compose file: %v", err)
		http.Error(w, "Failed to save docker-compose.yml", http.StatusInternalServerError)
		return
	}

	// Write .env file (can be empty)
	envPath := filepath.Join(appDetails.Path, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0640); err != nil { //nolint:gosec // Env file permissions appropriate
		logging.Errorf("Failed to write .env file: %v", err)
		// Don't fail the whole operation if .env fails
	}

	// Write app.yml file (can be empty)
	if appYmlContent != "" {
		appYmlPath := filepath.Join(appDetails.Path, "app.yml")
		if err := os.WriteFile(appYmlPath, []byte(appYmlContent), 0644); err != nil { //nolint:gosec // App yml needs to be readable
			logging.Errorf("Failed to write app.yml file: %v", err)
			// Don't fail the whole operation if app.yml fails
		}
	}

	// Check if container is running
	containerRunning := appDetails.Status == "running"

	// Get session for flash message
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
	}

	if containerRunning {
		session.AddFlash("Configuration saved. Please restart the container to apply changes.", "warning")
	} else {
		session.AddFlash("Configuration saved successfully.", "success")
	}

	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
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
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Cannot expose app: Caddy is not available. Please ensure Caddy is installed and running.", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get app details from container runtime
	appDetails, err := s.getAppDetails(appName)
	if err != nil {
		logging.Errorf("Failed to get app details: %v", err)
		session, sessErr := s.sessionStore.Get(r, "ontree-session")
		if sessErr != nil {
			logging.Errorf("Failed to get session: %v", sessErr)
		}
		message := "Failed to expose app: app not found"
		if errors.Is(err, errRuntimeUnavailable) {
			message = "Failed to expose app: container runtime not available"
		}
		session.AddFlash(message, "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		logging.Errorf("Failed to read compose metadata: %v", err)
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
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("App is already exposed", "info")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
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
										logging.Errorf("Failed to parse port from %s: %v", parts[0], err)
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

	// Use lowercase app name as ID for route
	appID := strings.ToLower(appName)
	logging.Infof("[Expose] Exposing app %s with subdomain %s on port %d", appName, metadata.Subdomain, metadata.HostPort)

	// Create route config (only for public domain, Tailscale handled separately)
	routeConfig := caddy.CreateRouteConfig(appID, metadata.Subdomain, metadata.HostPort, s.config.PublicBaseDomain, "")

	// Add route to Caddy
	logging.Infof("[Expose] Sending route config to Caddy for app %s", appName)
	err = s.caddyClient.AddOrUpdateRoute(routeConfig)
	if err != nil {
		logging.Errorf("[Expose] Failed to add route to Caddy: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash(fmt.Sprintf("Failed to expose app: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Update compose file metadata
	metadata.IsExposed = true
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		logging.Errorf("Failed to update compose metadata: %v", err)
		// Try to rollback Caddy change
		_ = s.caddyClient.DeleteRoute(fmt.Sprintf("route-for-%s", appID))
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to expose app: could not update metadata", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	logging.Infof("Successfully exposed app %s", appName)
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
	}

	publicURL := fmt.Sprintf("https://%s.%s", metadata.Subdomain, s.config.PublicBaseDomain)
	session.AddFlash(fmt.Sprintf("App exposed successfully at: %s", publicURL), "success")
	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
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

	// Get app details from container runtime
	appDetails, err := s.getAppDetails(appName)
	if err != nil {
		logging.Errorf("Failed to get app details: %v", err)
		session, sessErr := s.sessionStore.Get(r, "ontree-session")
		if sessErr != nil {
			logging.Errorf("Failed to get session: %v", sessErr)
		}
		message := "Failed to unexpose app: app not found"
		if errors.Is(err, errRuntimeUnavailable) {
			message = "Failed to unexpose app: container runtime not available"
		}
		session.AddFlash(message, "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		logging.Errorf("Failed to read compose metadata: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: could not read metadata", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Check if not exposed
	if !metadata.IsExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("App is not exposed", "info")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Delete route from Caddy if client is available
	if s.caddyClient != nil {
		appID := strings.ToLower(appName)
		routeID := fmt.Sprintf("route-for-%s", appID)
		err = s.caddyClient.DeleteRoute(routeID)
		if err != nil {
			logging.Errorf("Failed to delete route from Caddy: %v", err)
			// Continue anyway - we'll update the metadata
		}
	}

	// Update compose file metadata
	metadata.IsExposed = false
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		logging.Errorf("Failed to update compose metadata: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: could not update metadata", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	logging.Infof("Successfully unexposed app %s", appName)
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
	}
	session.AddFlash("App unexposed successfully", "success")
	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleSettings handles the settings page display
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())

	// Get current system setup
	var setup database.SystemSetup

	// Query the database
	var nodeIcon sql.NullString
	var nodeName sql.NullString
	err := s.db.QueryRow(`
		SELECT id, public_base_domain, tailscale_auth_key, tailscale_tags,
		       agent_enabled, agent_check_interval, agent_llm_api_key,
		       agent_llm_api_url, agent_llm_model,
		       uptime_kuma_base_url, update_channel, node_icon, node_name
		FROM system_setup
		WHERE id = 1
	`).Scan(&setup.ID, &setup.PublicBaseDomain, &setup.TailscaleAuthKey, &setup.TailscaleTags,
		&setup.AgentEnabled, &setup.AgentCheckInterval, &setup.AgentLLMAPIKey,
		&setup.AgentLLMAPIURL, &setup.AgentLLMModel,
		&setup.UptimeKumaBaseURL, &setup.UpdateChannel, &nodeIcon, &nodeName)

	if err != nil && err != sql.ErrNoRows {
		logging.Errorf("Failed to get system setup: %v", err)
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	// Get flash messages
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
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
		logging.Errorf("Failed to save session: %v", err)
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
	data["UpdateChannel"] = "stable" // Default to stable
	data["CurrentVersion"] = s.versionInfo.Version

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
	if setup.UpdateChannel.Valid {
		data["UpdateChannel"] = setup.UpdateChannel.String
	} else {
		// Default to stable if not set
		data["UpdateChannel"] = "stable"
	}

	// Also show current values from config (to show if env vars are overriding)
	data["ConfigPublicDomain"] = s.config.PublicBaseDomain
	data["ConfigTailscaleAuthKey"] = s.config.TailscaleAuthKey != ""
	data["ConfigTailscaleTags"] = s.config.TailscaleTags
	data["ConfigAgentLLMAPIKey"] = s.config.AgentLLMAPIKey != ""

	// Get completed models for dropdown
	completedModels, err := ollama.GetCompletedModels(s.db)
	if err != nil {
		logging.Errorf("Failed to get completed models: %v", err)
		completedModels = []ollama.OllamaModel{} // Empty list if error
	}
	data["CompletedModels"] = completedModels

	// Add current node icon
	if nodeIcon.Valid && nodeIcon.String != "" {
		data["CurrentNodeIcon"] = nodeIcon.String
	} else {
		data["CurrentNodeIcon"] = "tree1.png" // Default icon
	}
	data["SystemCheckAutoRun"] = false
	data["SystemCheckVisible"] = false
	data["SystemCheckPanelID"] = "system-check-settings"

	// Add current node name
	if nodeName.Valid && nodeName.String != "" {
		data["NodeName"] = nodeName.String
	} else {
		data["NodeName"] = "TreeOS" // Default name
	}

	// Render template
	tmpl, ok := s.templates["settings"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		logging.Errorf("Error rendering template: %v", err)
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

	// Check which action is being performed
	action := r.FormValue("action")

	// Ensure system_setup record exists
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO system_setup (id, is_setup_complete)
		VALUES (1, 1)
	`)
	if err != nil {
		logging.Errorf("Failed to ensure system_setup exists: %v", err)
	}

	// Handle different actions
	switch action {
	case "update_node_name":
		// Handle node name update
		nodeName := strings.TrimSpace(r.FormValue("node_name"))

		_, err = s.db.Exec(`
			UPDATE system_setup SET node_name = ? WHERE id = 1
		`, nodeName)

		if err != nil {
			logging.Errorf("Failed to update node name: %v", err)
			session, sessionErr := s.sessionStore.Get(r, "ontree-session")
			if sessionErr != nil {
				logging.Errorf("Failed to get session: %v", sessionErr)
			} else {
				session.AddFlash("Failed to save node name", "error")
				if saveErr := session.Save(r, w); saveErr != nil {
					logging.Errorf("Failed to save session: %v", saveErr)
				}
			}
		} else {
			// Success message
			session, sessionErr := s.sessionStore.Get(r, "ontree-session")
			if sessionErr != nil {
				logging.Errorf("Failed to get session: %v", sessionErr)
			} else {
				session.AddFlash("Node name updated successfully", "success")
				if saveErr := session.Save(r, w); saveErr != nil {
					logging.Errorf("Failed to save session: %v", saveErr)
				}
			}
		}

		http.Redirect(w, r, "/settings", http.StatusFound)
		return
	case "update_node_icon":
		// Handle node icon update
		nodeIcon := strings.TrimSpace(r.FormValue("node_icon"))

		// Default icon if none selected
		if nodeIcon == "" {
			nodeIcon = "tree1.png"
		}

		_, err = s.db.Exec(`
			UPDATE system_setup SET node_icon = ? WHERE id = 1
		`, nodeIcon)

		if err != nil {
			logging.Errorf("Failed to update node icon: %v", err)
			session, sessionErr := s.sessionStore.Get(r, "ontree-session")
			if sessionErr != nil {
				logging.Errorf("Failed to get session: %v", sessionErr)
			} else {
				session.AddFlash("Failed to save node icon", "error")
				if saveErr := session.Save(r, w); saveErr != nil {
					logging.Errorf("Failed to save session: %v", saveErr)
				}
			}
		} else {
			// Success message
			session, sessionErr := s.sessionStore.Get(r, "ontree-session")
			if sessionErr != nil {
				logging.Errorf("Failed to get session: %v", sessionErr)
			} else {
				session.AddFlash("Node icon updated successfully", "success")
				if saveErr := session.Save(r, w); saveErr != nil {
					logging.Errorf("Failed to save session: %v", saveErr)
				}
			}
		}

		http.Redirect(w, r, "/settings", http.StatusFound)
		return
	}

	// Original settings update logic for other forms
	publicDomain := strings.TrimSpace(r.FormValue("public_base_domain"))
	tailscaleAuthKey := strings.TrimSpace(r.FormValue("tailscale_auth_key"))
	tailscaleTags := strings.TrimSpace(r.FormValue("tailscale_tags"))
	uptimeKumaBaseURL := strings.TrimSpace(r.FormValue("uptime_kuma_base_url"))
	updateChannel := strings.TrimSpace(r.FormValue("update_channel"))
	nodeIcon := strings.TrimSpace(r.FormValue("node_icon"))

	// Default icon if none selected
	if nodeIcon == "" {
		nodeIcon = "tree1.png"
	}

	// Validate update channel
	if updateChannel != "stable" && updateChannel != "beta" {
		updateChannel = "beta" // Default to beta
	}

	// Handle agent type and model selection
	agentType := r.FormValue("agent_type")
	var agentLLMAPIKey, agentLLMAPIURL, agentLLMModel string

	switch agentType {
	case "local":
		// Local agent configuration
		agentLLMModel = strings.TrimSpace(r.FormValue("agent_llm_model_local"))
		agentLLMAPIURL = "http://localhost:11434/v1/chat/completions"
		agentLLMAPIKey = "" // Local doesn't need API key
	case "cloud":
		// Cloud agent configuration
		agentLLMAPIKey = strings.TrimSpace(r.FormValue("agent_llm_api_key"))
		agentLLMAPIURL = strings.TrimSpace(r.FormValue("agent_llm_api_url"))
		agentLLMModel = strings.TrimSpace(r.FormValue("agent_llm_model_cloud"))

		// Default to OpenAI if URL is empty
		if agentLLMAPIURL == "" {
			agentLLMAPIURL = "https://api.openai.com/v1/chat/completions"
		}
	}

	// Update database - try with update_channel and node_icon first
	_, err = s.db.Exec(`
		UPDATE system_setup
		SET public_base_domain = ?, tailscale_auth_key = ?, tailscale_tags = ?,
		    agent_llm_api_key = ?,
		    agent_llm_api_url = ?, agent_llm_model = ?,
		    uptime_kuma_base_url = ?, update_channel = ?, node_icon = ?
		WHERE id = 1
	`, publicDomain, tailscaleAuthKey, tailscaleTags,
		agentLLMAPIKey, agentLLMAPIURL, agentLLMModel,
		uptimeKumaBaseURL, updateChannel, nodeIcon)

	// If update_channel or node_icon columns don't exist, try without them
	if err != nil && (strings.Contains(err.Error(), "no such column: update_channel") || strings.Contains(err.Error(), "no such column: node_icon")) {
		_, err = s.db.Exec(`
			UPDATE system_setup
			SET public_base_domain = ?, tailscale_auth_key = ?, tailscale_tags = ?,
			    agent_llm_api_key = ?,
			    agent_llm_api_url = ?, agent_llm_model = ?,
			    uptime_kuma_base_url = ?
			WHERE id = 1
		`, publicDomain, tailscaleAuthKey, tailscaleTags,
			agentLLMAPIKey, agentLLMAPIURL, agentLLMModel,
			uptimeKumaBaseURL)
	}

	if err != nil {
		logging.Errorf("Failed to update settings: %v", err)
		session, sessionErr := s.sessionStore.Get(r, "ontree-session")
		if sessionErr != nil {
			logging.Errorf("Failed to get session: %v", sessionErr)
		} else {
			session.AddFlash("Failed to save settings", "error")
			if saveErr := session.Save(r, w); saveErr != nil {
				logging.Errorf("Failed to save session: %v", saveErr)
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

	// Success message
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
	} else {
		session.AddFlash("Settings saved successfully", "success")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
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

	// Get app details from container runtime
	appDetails, err := s.getAppDetails(appName)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		message := `<div class="alert alert-warning">App not found</div>`
		if errors.Is(err, errRuntimeUnavailable) {
			message = `<div class="alert alert-warning">Container runtime not available. Try again once Docker is running.</div>`
		}
		_, _ = w.Write([]byte(message))
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
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
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
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
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
	trimmed := strings.TrimSpace(output)
	if err != nil || trimmed == "" {
		// Fallback to alternative label format
		cmd = fmt.Sprintf(`docker ps --filter "name=%s" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"`, projectName)
		output, err = s.executeCommand(cmd)
		trimmed = strings.TrimSpace(output)
	}

	if err != nil || trimmed == "" {
		// If no containers found, check for single-service container by name
		cmd = fmt.Sprintf(`docker ps --filter "name=^%s$" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"`, projectName)
		output, err = s.executeCommand(cmd)
		trimmed = strings.TrimSpace(output)
		if err != nil || trimmed == "" {
			w.Write([]byte(`<div class="text-muted">No running containers found</div>`)) //nolint:errcheck,gosec // HTTP response
			return
		}
	}

	// Return the output wrapped in a pre tag for proper formatting
	html := fmt.Sprintf(`<pre class="mb-0" style="font-size: 0.875rem;">%s</pre>`, template.HTMLEscapeString(output))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html)) //nolint:errcheck,gosec // HTTP response
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
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Cannot expose app via Tailscale: Auth key not configured in settings", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get app details from container runtime
	appDetails, err := s.getAppDetails(appName)
	if err != nil {
		logging.Errorf("Failed to get app details: %v", err)
		session, sessErr := s.sessionStore.Get(r, "ontree-session")
		if sessErr != nil {
			logging.Errorf("Failed to get session: %v", sessErr)
		}
		message := "Failed to expose app: app not found"
		if errors.Is(err, errRuntimeUnavailable) {
			message = "Failed to expose app: container runtime not available"
		}
		session.AddFlash(message, "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		logging.Errorf("Failed to read compose metadata: %v", err)
		// Initialize with defaults if metadata doesn't exist
		metadata = &yamlutil.OnTreeMetadata{}
	}

	// Check if already exposed via Tailscale
	if metadata.TailscaleExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("App is already exposed via Tailscale", "info")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get hostname from form or default to app name
	hostname := r.FormValue("hostname")
	if hostname == "" {
		hostname = appName
	}

	logging.Infof("[Tailscale Expose] Exposing app %s with hostname %s", appName, hostname)

	// Modify compose file to add Tailscale sidecar
	err = yamlutil.ModifyComposeForTailscale(appDetails.Path, appName, hostname, s.config.TailscaleAuthKey)
	if err != nil {
		logging.Errorf("[Tailscale Expose] Failed to modify compose file: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash(fmt.Sprintf("Failed to expose app via Tailscale: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Update metadata
	metadata.TailscaleHostname = hostname
	metadata.TailscaleExposed = true
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		logging.Errorf("Failed to update compose metadata: %v", err)
	}

	// Restart containers with new configuration
	logging.Infof("[Tailscale Expose] Restarting containers for app %s", appName)
	cmd := fmt.Sprintf("cd '%s' && docker compose down && docker compose up -d", appDetails.Path)
	output, err := s.executeCommand(cmd)
	if err != nil {
		logging.Errorf("[Tailscale Expose] Failed to restart containers: %v, output: %s", err, output)
		// Try to rollback
		if err := yamlutil.RestoreComposeFromTailscale(appDetails.Path); err != nil {
			logging.Errorf("Failed to restore compose from Tailscale: %v", err)
		}
		metadata.TailscaleHostname = ""
		metadata.TailscaleExposed = false
		if err := yamlutil.UpdateComposeMetadata(appDetails.Path, metadata); err != nil {
			logging.Errorf("Failed to update compose metadata: %v", err)
		}

		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to restart containers after adding Tailscale", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Success
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
	}

	tailscaleURL := fmt.Sprintf("https://%s", hostname)
	if s.config.TailscaleTags != "" {
		tailscaleURL = fmt.Sprintf("https://%s (with tags: %s)", hostname, s.config.TailscaleTags)
	}

	session.AddFlash(fmt.Sprintf("App exposed via Tailscale at: %s", tailscaleURL), "success")
	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
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

	// Get app details from container runtime
	appDetails, err := s.getAppDetails(appName)
	if err != nil {
		logging.Errorf("Failed to get app details: %v", err)
		session, sessErr := s.sessionStore.Get(r, "ontree-session")
		if sessErr != nil {
			logging.Errorf("Failed to get session: %v", sessErr)
		}
		message := "Failed to unexpose app: app not found"
		if errors.Is(err, errRuntimeUnavailable) {
			message = "Failed to unexpose app: container runtime not available"
		}
		session.AddFlash(message, "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Get metadata from compose file
	metadata, err := yamlutil.ReadComposeMetadata(appDetails.Path)
	if err != nil {
		logging.Errorf("Failed to read compose metadata: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Failed to unexpose app: could not read metadata", "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Check if exposed via Tailscale
	if !metadata.TailscaleExposed {
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("App is not exposed via Tailscale", "info")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	logging.Infof("[Tailscale Unexpose] Removing Tailscale from app %s", appName)

	// Stop containers first
	cmd := fmt.Sprintf("cd '%s' && docker compose down", appDetails.Path)
	output, err := s.executeCommand(cmd)
	if err != nil {
		logging.Errorf("[Tailscale Unexpose] Warning: Failed to stop containers: %v, output: %s", err, output)
	}

	// Remove Tailscale sidecar from container runtime-compose.yml
	err = yamlutil.RestoreComposeFromTailscale(appDetails.Path)
	if err != nil {
		logging.Errorf("[Tailscale Unexpose] Failed to restore compose file: %v", err)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash(fmt.Sprintf("Failed to remove Tailscale: %v", err), "error")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Update metadata
	metadata.TailscaleHostname = ""
	metadata.TailscaleExposed = false
	err = yamlutil.UpdateComposeMetadata(appDetails.Path, metadata)
	if err != nil {
		logging.Errorf("Failed to update compose metadata: %v", err)
	}

	// Restart containers with original configuration
	logging.Infof("[Tailscale Unexpose] Restarting containers for app %s", appName)
	cmd = fmt.Sprintf("cd '%s' && docker compose up -d", appDetails.Path)
	output, err = s.executeCommand(cmd)
	if err != nil {
		logging.Errorf("[Tailscale Unexpose] Failed to restart containers: %v, output: %s", err, output)
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			logging.Errorf("Failed to get session: %v", err)
		}
		session.AddFlash("Warning: Tailscale removed but failed to restart containers", "warning")
		if err := session.Save(r, w); err != nil {
			logging.Errorf("Failed to save session: %v", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
		return
	}

	// Success
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
	}

	session.AddFlash("App removed from Tailscale successfully", "success")
	if err := session.Save(r, w); err != nil {
		logging.Errorf("Failed to save session: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
}

// handleTestLLMConnection handles POST /api/test-llm requests
func (s *Server) handleTestLLMConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		APIKey string `json:"api_key"`
		APIURL string `json:"api_url"`
		Model  string `json:"model"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request format",
		}); err != nil {
			logging.Errorf("Error encoding response: %v", err)
		}
		return
	}

	// Test the connection
	response, err := s.testLLMConnection(req.APIKey, req.APIURL, req.Model)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusOK) // Return 200 even on error for better UX
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}); err != nil {
			logging.Errorf("Error encoding response: %v", err)
		}
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"response": response,
	}); err != nil {
		logging.Errorf("Error encoding response: %v", err)
	}
}
