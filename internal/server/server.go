package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"ontree-node/internal/config"
)

// Server represents the HTTP server
type Server struct {
	config    *config.Config
	templates map[string]*template.Template
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	s := &Server{
		config:    cfg,
		templates: make(map[string]*template.Template),
	}

	// Load templates
	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return s, nil
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

	return nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Set up routes
	mux := http.NewServeMux()

	// Static file serving
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Dashboard route
	mux.HandleFunc("/", s.handleDashboard)

	// Start server
	addr := s.config.ListenAddr
	if addr == "" {
		addr = ":8080"
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

	// Prepare template data
	data := struct {
		User      interface{}
		Apps      []interface{}
		AppsDir   string
		Messages  []interface{}
		CSRFToken string
	}{
		User:      nil, // No authentication yet
		Apps:      nil, // No apps yet
		AppsDir:   s.config.AppsDir,
		Messages:  nil,
		CSRFToken: "", // No CSRF yet
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