package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

// handleTemplates handles the templates list page
func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from context
	user := getUserFromContext(r.Context())

	// Get available templates
	templates, err := s.templateSvc.GetAvailableTemplates()
	if err != nil {
		log.Printf("Error getting templates: %v", err)
		http.Error(w, "Failed to load templates", http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG: Loaded %d templates", len(templates))
	for i, t := range templates {
		log.Printf("DEBUG: Template %d: %s (%s)", i, t.Name, t.Filename)
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["Templates"] = templates
	data["Messages"] = nil
	data["CSRFToken"] = "" // No CSRF yet

	// Render template
	tmpl, ok := s.templates["app_templates"]
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

// routeTemplates handles all /templates/* routes
func (s *Server) routeTemplates(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Parse template ID from path like /templates/openwebui/create
	parts := strings.Split(strings.TrimPrefix(path, "/templates/"), "/")
	if len(parts) >= 2 && parts[1] == "create" {
		templateID := parts[0]
		s.handleCreateFromTemplate(w, r, templateID)
	} else {
		http.NotFound(w, r)
	}
}

// handleCreateFromTemplate handles the create app from template page
func (s *Server) handleCreateFromTemplate(w http.ResponseWriter, r *http.Request, templateID string) {
	// Get the template
	template, err := s.templateSvc.GetTemplateByID(templateID)
	if err != nil {
		log.Printf("Error getting template %s: %v", templateID, err)
		http.NotFound(w, r)
		return
	}

	user := getUserFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		// Show the form
		data := s.baseTemplateData(user)
		data["Template"] = template
		data["Messages"] = nil
		data["CSRFToken"] = "" // No CSRF yet

		tmpl, ok := s.templates["app_create_from_template"]
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

	case http.MethodPost:
		// Handle form submission
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		appName := strings.TrimSpace(r.FormValue("name"))
		autoStart := r.FormValue("auto_start") == "on"

		// Validate app name
		if appName == "" {
			http.Error(w, "Application name is required", http.StatusBadRequest)
			return
		}

		// Get template content
		content, err := s.templateSvc.GetTemplateContent(template)
		if err != nil {
			log.Printf("Error getting template content: %v", err)
			http.Error(w, "Failed to read template", http.StatusInternalServerError)
			return
		}

		// Process template content (replace service name)
		processedContent := s.templateSvc.ProcessTemplateContent(content, appName)

		// Create the app using existing scaffold logic
		if err := s.createAppScaffold(appName, processedContent); err != nil {
			log.Printf("Error creating app from template: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create application: %v", err), http.StatusInternalServerError)
			return
		}

		// If auto-start is enabled, queue a start operation
		if autoStart && s.worker != nil {
			metadata := map[string]string{
				"template_id": template.ID,
				"auto_start":  "true",
			}
			operationID, err := s.createDockerOperation("start_container", appName, metadata)
			if err != nil {
				log.Printf("Error creating operation: %v", err)
			} else {
				s.worker.EnqueueOperation(operationID)
			}
		}

		// Redirect to app detail page
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
