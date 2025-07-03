package server

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"ontree-node/internal/config"
	"ontree-node/internal/database"
	"ontree-node/internal/docker"
	"ontree-node/internal/templates"
	"ontree-node/internal/worker"
	"github.com/gorilla/sessions"
)

// Server represents the HTTP server
type Server struct {
	config       *config.Config
	templates    map[string]*template.Template
	sessionStore *sessions.CookieStore
	dockerClient *docker.Client
	dockerSvc    *docker.Service
	db           *sql.DB
	worker       *worker.Worker
	templateSvc  *templates.Service
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	// Create session store with secure key
	// In production, this should be loaded from environment or config
	sessionKey := []byte("your-32-byte-session-key-here!!") // TODO: Load from config
	
	s := &Server{
		config:       cfg,
		templates:    make(map[string]*template.Template),
		sessionStore: sessions.NewCookieStore(sessionKey),
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
		s.dockerSvc.Close()
	}
	if s.dockerClient != nil {
		s.dockerClient.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
}

// loadTemplates loads all HTML templates
func (s *Server) loadTemplates() error {
	// Load base template
	baseTemplate := filepath.Join("templates", "layouts", "base.html")
	
	// Load dashboard template
	dashboardTemplate := filepath.Join("templates", "dashboard", "index.html")
	tmpl, err := template.ParseFiles(baseTemplate, dashboardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse dashboard template: %w", err)
	}
	s.templates["dashboard"] = tmpl

	// Load setup template
	setupTemplate := filepath.Join("templates", "dashboard", "setup.html")
	tmpl, err = template.ParseFiles(baseTemplate, setupTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse setup template: %w", err)
	}
	s.templates["setup"] = tmpl

	// Load login template
	loginTemplate := filepath.Join("templates", "dashboard", "login.html")
	tmpl, err = template.ParseFiles(baseTemplate, loginTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse login template: %w", err)
	}
	s.templates["login"] = tmpl

	// Load app detail template
	appDetailTemplate := filepath.Join("templates", "dashboard", "app_detail.html")
	tmpl, err = template.ParseFiles(baseTemplate, appDetailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app detail template: %w", err)
	}
	s.templates["app_detail"] = tmpl

	// Load app create template
	appCreateTemplate := filepath.Join("templates", "dashboard", "app_create.html")
	tmpl, err = template.ParseFiles(baseTemplate, appCreateTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app create template: %w", err)
	}
	s.templates["app_create"] = tmpl

	// Load app templates list template
	appTemplatesTemplate := filepath.Join("templates", "dashboard", "app_templates.html")
	tmpl, err = template.ParseFiles(baseTemplate, appTemplatesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app templates template: %w", err)
	}
	s.templates["app_templates"] = tmpl

	// Load app create from template template
	appCreateFromTemplate := filepath.Join("templates", "dashboard", "app_create_from_template.html")
	tmpl, err = template.ParseFiles(baseTemplate, appCreateFromTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app create from template template: %w", err)
	}
	s.templates["app_create_from_template"] = tmpl

	// Load pattern library templates
	// Pattern library index
	patternsIndexTemplate := filepath.Join("templates", "pattern_library", "index.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsIndexTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns index template: %w", err)
	}
	s.templates["patterns_index"] = tmpl

	// Pattern library components
	patternsComponentsTemplate := filepath.Join("templates", "pattern_library", "components.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsComponentsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns components template: %w", err)
	}
	s.templates["patterns_components"] = tmpl

	// Pattern library forms
	patternsFormsTemplate := filepath.Join("templates", "pattern_library", "forms.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsFormsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns forms template: %w", err)
	}
	s.templates["patterns_forms"] = tmpl

	// Pattern library typography
	patternsTypographyTemplate := filepath.Join("templates", "pattern_library", "typography.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsTypographyTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns typography template: %w", err)
	}
	s.templates["patterns_typography"] = tmpl

	// Pattern library partials
	patternsPartialsTemplate := filepath.Join("templates", "pattern_library", "partials.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsPartialsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns partials template: %w", err)
	}
	s.templates["patterns_partials"] = tmpl

	// Pattern library layouts
	patternsLayoutsTemplate := filepath.Join("templates", "pattern_library", "layouts.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsLayoutsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns layouts template: %w", err)
	}
	s.templates["patterns_layouts"] = tmpl

	// Pattern library style guide
	patternsStyleGuideTemplate := filepath.Join("templates", "pattern_library", "style_guide.html")
	tmpl, err = template.ParseFiles(baseTemplate, patternsStyleGuideTemplate)
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

	// Static file serving
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Public routes (no auth required)
	mux.HandleFunc("/setup", s.SetupRequiredMiddleware(s.handleSetup))
	mux.HandleFunc("/login", s.SetupRequiredMiddleware(s.handleLogin))
	mux.HandleFunc("/logout", s.handleLogout)

	// Protected routes (auth required)
	mux.HandleFunc("/", s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleDashboard)))
	mux.HandleFunc("/apps/", s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeApps)))
	mux.HandleFunc("/templates", s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleTemplates)))
	mux.HandleFunc("/templates/", s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeTemplates)))
	
	// API routes
	mux.HandleFunc("/api/system-vitals", s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleSystemVitals)))
	mux.HandleFunc("/api/docker/operations/", s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleDockerOperationStatus)))

	// Pattern library routes (no auth required - public access)
	mux.HandleFunc("/patterns", s.routePatterns)
	mux.HandleFunc("/patterns/", s.routePatterns)

	// Start server
	addr := s.config.ListenAddr
	if addr == "" {
		addr = ":8083"
	}

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, mux)
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
	data := struct {
		User        interface{}
		UserInitial string
		Apps        []interface{}
		AppsDir     string
		Messages    []interface{}
		CSRFToken   string
	}{
		User:        user,
		UserInitial: getUserInitial(user.Username),
		Apps:        apps,
		AppsDir:     s.config.AppsDir,
		Messages:    nil,
		CSRFToken:   "", // No CSRF yet
	}

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
	} else {
		// Default to app detail page
		s.handleAppDetail(w, r)
	}
}