package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func getLocalOllamaModels() []string {
	type tag struct {
		Name string `json:"name"`
	}

	type response struct {
		Models []tag `json:"models"`
	}

	endpoints := []string{
		"http://localhost:11434/api/tags",
		"http://127.0.0.1:11434/api/tags",
		"http://host.containers.internal:11434/api/tags",
	}

	client := &http.Client{Timeout: 2 * time.Second}

	for _, endpoint := range endpoints {
		resp, err := client.Get(endpoint)
		if err != nil {
			continue
		}

		var data response

		func() {
			defer resp.Body.Close() //nolint:errcheck // Cleanup, error not critical
			if resp.StatusCode != http.StatusOK {
				return
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				data.Models = nil
			}
		}()

		if len(data.Models) == 0 {
			continue
		}

		models := make([]string, 0, len(data.Models))
		for _, m := range data.Models {
			name := strings.TrimSpace(m.Name)
			if name != "" {
				models = append(models, name)
			}
		}

		if len(models) > 0 {
			return models
		}
	}

	return nil
}

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

	// Group templates by category with proper ordering
	categorizedTemplates := make(map[string][]interface{})
	categoryOrder := []string{"LLM Inference", "LLM Web Interfaces", "Others"}

	// Initialize categories in order
	for _, cat := range categoryOrder {
		categorizedTemplates[cat] = []interface{}{}
	}

	// Group templates
	for _, template := range templates {
		if _, exists := categorizedTemplates[template.Category]; exists {
			categorizedTemplates[template.Category] = append(categorizedTemplates[template.Category], template)
		} else {
			// If category doesn't exist, add to Others
			categorizedTemplates["Others"] = append(categorizedTemplates["Others"], template)
		}
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["CategorizedTemplates"] = categorizedTemplates
	data["CategoryOrder"] = categoryOrder
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

		// Process template content first (replace placeholders)
		processedContent := s.templateSvc.ProcessTemplateContent(content, appName)

		// If custom port is provided, replace ports in the processed YAML
		if customPort != "" {
			processedContent, err = s.replacePortsInYAML(processedContent, customPort)
			if err != nil {
				log.Printf("Error replacing ports in YAML: %v", err)
				http.Error(w, "Failed to update port configuration", http.StatusInternalServerError)
				return
			}
		}

		// Get .env.example content if it exists for this template
		envContent, err := s.templateSvc.GetTemplateEnvExample(templateID)
		if err != nil {
			log.Printf("Error reading .env.example for template %s: %v", templateID, err)
			http.Error(w, "Failed to read template environment file", http.StatusInternalServerError)
			return
		}
		if envContent != "" {
			log.Printf("Found .env.example for template %s, will use default environment variables", templateID)
		}

		// Create the app using scaffold logic with template flag
		if err := s.createAppScaffoldFromTemplate(appName, processedContent, envContent, emoji); err != nil {
			log.Printf("Error creating app from template: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create application: %v", err), http.StatusInternalServerError)
			return
		}

		// Special handling for LibreChat - copy config file
		if templateID == "librechat" {
			appPath := filepath.Join(s.config.AppsDir, appName)
			configDir := filepath.Join(appPath, "shared", "config")
			if err := os.MkdirAll(configDir, 0755); err != nil { //nolint:gosec // Config directory needs group read access
				log.Printf("Warning: Failed to create config directory for %s: %v", appName, err)
			} else {
				configPath := filepath.Join(configDir, "librechat.yaml")

				// Create default LibreChat config for Ollama integration
				models := getLocalOllamaModels()
				if len(models) == 0 {
					models = []string{"llama3.2:latest"}
				}
				var modelLines strings.Builder
				for _, model := range models {
					modelLines.WriteString(fmt.Sprintf("          - \"%s\"\n", model))
				}

				librechatConfig := fmt.Sprintf(`# LibreChat Configuration for Ollama Integration
version: 1.0.0

endpoints:
  custom:
    - name: "Ollama"
      apiKey: "ollama"
      baseURL: "http://host.containers.internal:11434/v1/"
      models:
        fetch: true
        default:
%s      titleConvo: true
      titleModel: "current_model"
      summarize: false
      summaryModel: "current_model"
      forcePrompt: false
      modelDisplayLabel: "Ollama"
      dropParams: ["stop"]`, modelLines.String())

				// #nosec G306 -- LibreChat needs this config mounted read-only inside the container
				if err := os.WriteFile(configPath, []byte(librechatConfig), 0644); err != nil {
					log.Printf("Warning: Failed to create librechat.yaml for %s: %v", appName, err)
				}
			}
		}

		// Trigger agent immediately for initial setup
		go s.triggerAgentForApp(appName)

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
					containerPort := strings.TrimSpace(parts[len(parts)-1])
					containerPort = strings.Trim(containerPort, "}\"")
					portsArray[i] = fmt.Sprintf("%s:%s", customPort, containerPort)
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
