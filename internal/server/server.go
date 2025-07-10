package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"ontree-node/internal/caddy"
	"ontree-node/internal/config"
	"ontree-node/internal/database"
	"ontree-node/internal/docker"
	"ontree-node/internal/embeds"
	"ontree-node/internal/templates"
	"ontree-node/internal/version"
	"ontree-node/internal/worker"
	"ontree-node/internal/yamlutil"
)

// Server represents the HTTP server
type Server struct {
	config                *config.Config
	templates             map[string]*template.Template
	sessionStore          *sessions.CookieStore
	dockerClient          *docker.Client
	dockerSvc             *docker.Service
	db                    *sql.DB
	worker                *worker.Worker
	templateSvc           *templates.Service
	versionInfo           version.Info
	caddyAvailable        bool
	caddyClient           *caddy.Client
	platformSupportsCaddy bool
}

// New creates a new server instance
func New(cfg *config.Config, versionInfo version.Info) (*Server, error) {
	// Create session store with secure key
	// In production, this should be loaded from environment or config
	sessionKey := []byte("your-32-byte-session-key-here!!") // TODO: Load from config

	s := &Server{
		config:                cfg,
		templates:             make(map[string]*template.Template),
		sessionStore:          sessions.NewCookieStore(sessionKey),
		versionInfo:           versionInfo,
		platformSupportsCaddy: runtime.GOOS == "linux",
	}

	// Configure session store
	s.sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	// Load templates
	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	s.db = db

	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Printf("Warning: Failed to initialize Docker client: %v", err)
		// Continue without Docker support
	} else {
		s.dockerClient = dockerClient
	}

	// Initialize Docker service
	dockerSvc, err := docker.NewService(cfg.AppsDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize Docker service: %v", err)
		// Continue without Docker support
	} else {
		s.dockerSvc = dockerSvc
	}

	// Load domain configuration from database if not set by environment
	if err := s.loadDomainConfig(); err != nil {
		log.Printf("Warning: Failed to load domain config from database: %v", err)
	}

	// Initialize Caddy client only on Linux
	if s.platformSupportsCaddy {
		s.caddyClient = caddy.NewClient()
		// Check Caddy availability
		s.checkCaddyHealth()
	} else {
		log.Printf("Caddy integration is not supported on %s platform", runtime.GOOS)
		s.caddyAvailable = false
	}

	// Initialize worker if Docker is available
	if s.dockerSvc != nil && s.db != nil {
		s.worker = worker.New(s.db, s.dockerSvc)
		// Start workers (using 2 workers for now)
		s.worker.Start(2)
	}

	// Initialize template service
	templatesPath := filepath.Join("templates", "compose")
	s.templateSvc = templates.NewService(templatesPath)

	return s, nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	if s.worker != nil {
		s.worker.Stop()
	}
	if s.dockerSvc != nil {
		if err := s.dockerSvc.Close(); err != nil {
			log.Printf("Error closing docker service: %v", err)
		}
	}
	if s.dockerClient != nil {
		if err := s.dockerClient.Close(); err != nil {
			log.Printf("Error closing docker client: %v", err)
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
}

// loadTemplates loads all HTML templates
func (s *Server) loadTemplates() error {
	// Load base template
	baseTemplate := filepath.Join("templates", "layouts", "base.html")

	// Load dashboard template
	dashboardTemplate := filepath.Join("templates", "dashboard", "index.html")
	tmpl, err := embeds.ParseTemplate(baseTemplate, dashboardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse dashboard template: %w", err)
	}
	s.templates["dashboard"] = tmpl

	// Load setup template
	setupTemplate := filepath.Join("templates", "dashboard", "setup.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, setupTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse setup template: %w", err)
	}
	s.templates["setup"] = tmpl

	// Load login template
	loginTemplate := filepath.Join("templates", "dashboard", "login.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, loginTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse login template: %w", err)
	}
	s.templates["login"] = tmpl

	// Load settings template
	settingsTemplate := filepath.Join("templates", "dashboard", "settings.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, settingsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse settings template: %w", err)
	}
	s.templates["settings"] = tmpl

	// Load app detail template
	appDetailTemplate := filepath.Join("templates", "dashboard", "app_detail.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appDetailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app detail template: %w", err)
	}
	s.templates["app_detail"] = tmpl

	// Load app create template
	appCreateTemplate := filepath.Join("templates", "dashboard", "app_create.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appCreateTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app create template: %w", err)
	}
	s.templates["app_create"] = tmpl

	// Load app templates list template
	appTemplatesTemplate := filepath.Join("templates", "dashboard", "app_templates.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appTemplatesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app templates template: %w", err)
	}
	s.templates["app_templates"] = tmpl

	// Load app create from template template
	appCreateFromTemplate := filepath.Join("templates", "dashboard", "app_create_from_template.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appCreateFromTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app create from template template: %w", err)
	}
	s.templates["app_create_from_template"] = tmpl

	// Load pattern library templates
	// Pattern library index
	patternsIndexTemplate := filepath.Join("templates", "pattern_library", "index.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsIndexTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns index template: %w", err)
	}
	s.templates["patterns_index"] = tmpl

	// Pattern library components
	patternsComponentsTemplate := filepath.Join("templates", "pattern_library", "components.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsComponentsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns components template: %w", err)
	}
	s.templates["patterns_components"] = tmpl

	// Pattern library forms
	patternsFormsTemplate := filepath.Join("templates", "pattern_library", "forms.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsFormsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns forms template: %w", err)
	}
	s.templates["patterns_forms"] = tmpl

	// Pattern library typography
	patternsTypographyTemplate := filepath.Join("templates", "pattern_library", "typography.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsTypographyTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns typography template: %w", err)
	}
	s.templates["patterns_typography"] = tmpl

	// Pattern library partials
	patternsPartialsTemplate := filepath.Join("templates", "pattern_library", "partials.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsPartialsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns partials template: %w", err)
	}
	s.templates["patterns_partials"] = tmpl

	// Pattern library layouts
	patternsLayoutsTemplate := filepath.Join("templates", "pattern_library", "layouts.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsLayoutsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns layouts template: %w", err)
	}
	s.templates["patterns_layouts"] = tmpl

	// Pattern library style guide
	patternsStyleGuideTemplate := filepath.Join("templates", "pattern_library", "style_guide.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsStyleGuideTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns style guide template: %w", err)
	}
	s.templates["patterns_style_guide"] = tmpl

	return nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Set up routes
	mux := http.NewServeMux()

	// Static file serving using embedded files
	staticFS, err := embeds.StaticFS()
	if err != nil {
		return fmt.Errorf("failed to get static filesystem: %w", err)
	}
	fs := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Public routes (no auth required)
	mux.HandleFunc("/setup", s.TracingMiddleware(s.SetupRequiredMiddleware(s.handleSetup)))
	mux.HandleFunc("/login", s.TracingMiddleware(s.SetupRequiredMiddleware(s.handleLogin)))
	mux.HandleFunc("/logout", s.TracingMiddleware(s.handleLogout))

	// Protected routes (auth required)
	mux.HandleFunc("/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleDashboard))))
	mux.HandleFunc("/apps/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeApps))))
	mux.HandleFunc("/templates", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleTemplates))))
	mux.HandleFunc("/templates/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeTemplates))))

	// API routes
	mux.HandleFunc("/api/system-vitals", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleSystemVitals))))
	mux.HandleFunc("/api/docker/operations/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeDockerOperations))))
	mux.HandleFunc("/api/apps/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeAPIApps))))

	// Version endpoint (no auth required for automation/monitoring)
	mux.HandleFunc("/version", s.TracingMiddleware(s.handleVersion))

	// Pattern library routes (no auth required - public access)
	mux.HandleFunc("/patterns", s.TracingMiddleware(s.routePatterns))
	mux.HandleFunc("/patterns/", s.TracingMiddleware(s.routePatterns))

	// Settings routes
	mux.HandleFunc("/settings", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			s.handleSettingsUpdate(w, r)
		} else {
			s.handleSettings(w, r)
		}
	}))))

	// Start server
	addr := s.config.ListenAddr
	if addr == "" {
		addr = config.DefaultPort
	}

	log.Printf("Starting server on %s", addr)

	// Create server with proper timeouts
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// handleDashboard handles the dashboard page
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Only handle exact path match
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get user from context
	user := getUserFromContext(r.Context())

	// Scan for applications
	var apps []interface{}
	if s.dockerClient != nil {
		dockerApps, err := s.dockerClient.ScanApps(s.config.AppsDir)
		if err != nil {
			log.Printf("Error scanning apps: %v", err)
		} else {
			// Convert to interface{} slice for template
			for _, app := range dockerApps {
				apps = append(apps, app)
			}
		}
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["Apps"] = apps
	data["AppsDir"] = s.config.AppsDir
	data["Messages"] = nil
	data["CSRFToken"] = "" // No CSRF yet

	// Render template
	tmpl, ok := s.templates["dashboard"]
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

// getUserInitial gets the first letter of username in uppercase
func getUserInitial(username string) string {
	if username == "" {
		return "?"
	}
	return strings.ToUpper(string(username[0]))
}

// baseTemplateData creates the common data structure for base template
func (s *Server) baseTemplateData(user *database.User) map[string]interface{} {
	data := make(map[string]interface{})

	if user != nil {
		data["User"] = user
		data["UserInitial"] = getUserInitial(user.Username)

		// PostHog configuration
		if s.config.PostHogAPIKey != "" {
			data["PostHogEnabled"] = true
			data["PostHogAPIKey"] = s.config.PostHogAPIKey
			data["PostHogHost"] = s.config.PostHogHost
		}
	}

	// Version info
	data["Version"] = s.versionInfo.Version
	data["VersionAge"] = version.GetVersionAge()

	// Caddy availability
	data["CaddyAvailable"] = s.caddyAvailable
	data["PlatformSupportsCaddy"] = s.platformSupportsCaddy

	// Messages field is required by base template
	data["Messages"] = nil

	return data
}

// checkCaddyHealth checks if Caddy is available and running
func (s *Server) checkCaddyHealth() {
	// Skip Caddy checks on non-Linux platforms
	if !s.platformSupportsCaddy {
		s.caddyAvailable = false
		return
	}

	if s.caddyClient == nil {
		s.caddyAvailable = false
		return
	}

	err := s.caddyClient.HealthCheck()
	if err != nil {
		log.Printf("Cannot connect to Caddy Admin API at localhost:2019. Please ensure Caddy is installed and running. Error: %v", err)
		s.caddyAvailable = false
		return
	}

	log.Printf("Successfully connected to Caddy Admin API")
	s.caddyAvailable = true

	// Sync exposed apps if database is available
	if s.db != nil && s.caddyAvailable {
		s.syncExposedApps()
	}
}

// loadDomainConfig loads domain configuration from database if not set by environment
func (s *Server) loadDomainConfig() error {
	// Skip if environment variables are set
	if os.Getenv("PUBLIC_BASE_DOMAIN") != "" && os.Getenv("TAILSCALE_BASE_DOMAIN") != "" {
		return nil
	}

	// Query database for domain configuration
	var publicDomain, tailscaleDomain sql.NullString
	err := s.db.QueryRow(`
		SELECT public_base_domain, tailscale_base_domain 
		FROM system_setup 
		WHERE id = 1
	`).Scan(&publicDomain, &tailscaleDomain)

	if err != nil {
		if err == sql.ErrNoRows {
			// No config yet, that's OK
			return nil
		}
		return fmt.Errorf("failed to query domain config: %w", err)
	}

	// Update config if not overridden by environment
	if os.Getenv("PUBLIC_BASE_DOMAIN") == "" && publicDomain.Valid {
		s.config.PublicBaseDomain = publicDomain.String
	}
	if os.Getenv("TAILSCALE_BASE_DOMAIN") == "" && tailscaleDomain.Valid {
		s.config.TailscaleBaseDomain = tailscaleDomain.String
	}

	return nil
}

// syncExposedApps synchronizes exposed apps with Caddy on startup
func (s *Server) syncExposedApps() {
	// Read all apps from the apps directory
	apps, err := s.dockerSvc.ScanApps()
	if err != nil {
		log.Printf("Failed to list apps: %v", err)
		return
	}

	// Get base domains from config
	publicDomain := s.config.PublicBaseDomain
	tailscaleDomain := s.config.TailscaleBaseDomain

	for _, app := range apps {
		// Read metadata from compose file
		metadata, err := yamlutil.ReadComposeMetadata(app.Path)
		if err != nil {
			log.Printf("Failed to read metadata for app %s: %v", app.Name, err)
			continue
		}

		// Skip if not exposed
		if metadata == nil || !metadata.IsExposed || metadata.Subdomain == "" || metadata.HostPort == 0 {
			continue
		}

		// Generate ID for Caddy route
		appID := fmt.Sprintf("app-%s", app.Name)

		// Create route config
		routeConfig := caddy.CreateRouteConfig(appID, metadata.Subdomain, metadata.HostPort, publicDomain, tailscaleDomain)

		// Add route to Caddy
		err = s.caddyClient.AddOrUpdateRoute(routeConfig)
		if err != nil {
			log.Printf("Failed to sync app %s to Caddy: %v", app.Name, err)
		} else {
			log.Printf("Successfully synced app %s to Caddy", app.Name)
		}
	}
}

// routeApps routes all /apps/* requests to the appropriate handler
func (s *Server) routeApps(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	if path == "/apps/create" {
		s.handleAppCreate(w, r)
	} else if strings.HasSuffix(path, "/start") {
		s.handleAppStart(w, r)
	} else if strings.HasSuffix(path, "/stop") {
		s.handleAppStop(w, r)
	} else if strings.HasSuffix(path, "/recreate") {
		s.handleAppRecreate(w, r)
	} else if strings.HasSuffix(path, "/delete") {
		s.handleAppDelete(w, r)
	} else if strings.HasSuffix(path, "/check-update") {
		s.handleAppCheckUpdate(w, r)
	} else if strings.HasSuffix(path, "/update") {
		s.handleAppUpdate(w, r)
	} else if strings.HasSuffix(path, "/controls") {
		s.handleAppControls(w, r)
	} else if strings.HasSuffix(path, "/expose") {
		s.handleAppExpose(w, r)
	} else if strings.HasSuffix(path, "/unexpose") {
		s.handleAppUnexpose(w, r)
	} else {
		// Default to app detail page
		s.handleAppDetail(w, r)
	}
}

// routeAPIApps routes /api/apps/* requests
func (s *Server) routeAPIApps(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	if strings.HasSuffix(path, "/status") {
		s.handleAppStatusCheck(w, r)
	} else {
		http.NotFound(w, r)
	}
}

// routeDockerOperations routes /api/docker/operations/* requests
func (s *Server) routeDockerOperations(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	if strings.HasSuffix(path, "/logs") {
		s.handleDockerOperationLogs(w, r)
	} else {
		// Default to operation status
		s.handleDockerOperationStatus(w, r)
	}
}

// handleVersion returns version information as JSON
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	versionData := map[string]interface{}{
		"version":   s.versionInfo.Version,
		"commit":    s.versionInfo.Commit,
		"buildDate": s.versionInfo.BuildDate,
		"goVersion": s.versionInfo.GoVersion,
		"compiler":  s.versionInfo.Compiler,
		"platform":  s.versionInfo.Platform,
	}

	if err := json.NewEncoder(w).Encode(versionData); err != nil {
		log.Printf("Error encoding version response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
