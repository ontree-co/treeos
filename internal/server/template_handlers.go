package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	
	"gopkg.in/yaml.v3"
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
		data["Emojis"] = getRandomEmojis(7)
		data["SelectedEmoji"] = ""

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
		emoji := strings.TrimSpace(r.FormValue("emoji"))
		customPort := strings.TrimSpace(r.FormValue("port"))

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

		// If custom port is provided, replace ports in the YAML
		if customPort != "" {
			content, err = s.replacePortsInYAML(content, customPort)
			if err != nil {
				log.Printf("Error replacing ports in YAML: %v", err)
				http.Error(w, "Failed to update port configuration", http.StatusInternalServerError)
				return
			}
		}

		// Process template content (for any other replacements)
		processedContent := s.templateSvc.ProcessTemplateContent(content, appName)

		// Create the app using existing scaffold logic
		if err := s.createAppScaffold(appName, processedContent, emoji); err != nil {
			log.Printf("Error creating app from template: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create application: %v", err), http.StatusInternalServerError)
			return
		}

		// Auto-start is no longer supported - users must manually start apps

		// Redirect to app detail page
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// replacePortsInYAML replaces all host ports in the docker-compose YAML with the custom port
func (s *Server) replacePortsInYAML(content string, customPort string) (string, error) {
	// Validate port number
	port, err := strconv.Atoi(customPort)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port number: %s", customPort)
	}

	// Parse YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Get services
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return content, nil // No services, return as-is
	}

	// Process each service
	for _, service := range services {
		serviceMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for ports
		ports, ok := serviceMap["ports"]
		if !ok {
			continue
		}

		// Process ports array
		portsArray, ok := ports.([]interface{})
		if !ok {
			continue
		}

		// Replace host port in each port mapping
		for i, portEntry := range portsArray {
			portStr, ok := portEntry.(string)
			if !ok {
				continue
			}

			// Parse port mapping (e.g., "4000:8080" or just "8080")
			if strings.Contains(portStr, ":") {
				parts := strings.Split(portStr, ":")
				if len(parts) >= 2 {
					// Replace host port, keep container port
					portsArray[i] = fmt.Sprintf("%s:%s", customPort, parts[1])
				}
			} else {
				// Single port format, replace with explicit mapping
				portsArray[i] = fmt.Sprintf("%s:%s", customPort, portStr)
			}
		}
	}

	// Marshal back to YAML
	output, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}

	return string(output), nil
}
