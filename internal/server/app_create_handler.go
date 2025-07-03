package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"gopkg.in/yaml.v3"
)

// handleAppCreate handles the application creation page
func (s *Server) handleAppCreate(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user := getUserFromContext(r.Context())
	
	if r.Method == "POST" {
		// Parse form
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		
		appName := r.FormValue("app_name")
		composeContent := r.FormValue("compose_content")
		
		// Validate
		var errors []string
		
		// Validate app name
		if appName == "" {
			errors = append(errors, "App name is required")
		} else if !isValidAppName(appName) {
			errors = append(errors, "Invalid app name. Use only letters, numbers, hyphens, and underscores")
		} else if s.dockerClient != nil {
			// Check if app already exists
			appPath := filepath.Join(s.config.AppsDir, appName)
			if _, err := os.Stat(appPath); err == nil {
				errors = append(errors, fmt.Sprintf("An application named '%s' already exists", appName))
			}
		}
		
		// Validate compose content
		if composeContent == "" {
			errors = append(errors, "Docker compose content cannot be empty")
		} else {
			// Validate YAML and service name
			if err := validateComposeContent(composeContent, appName); err != nil {
				errors = append(errors, err.Error())
			}
		}
		
		if len(errors) == 0 && s.dockerClient != nil {
			// Create the application
			err := createAppScaffold(s.config.AppsDir, appName, composeContent)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to create application: %v", err))
			} else {
				log.Printf("Successfully created application: %s", appName)
				
				// Set success message
				session, _ := s.sessionStore.Get(r, "ontree-session")
				session.AddFlash(fmt.Sprintf("Application '%s' has been created successfully! You can now manage it from the app detail page.", appName), "success")
				session.Save(r, w)
				
				// Redirect to app detail page
				http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
				return
			}
		}
		
		// Render with errors
		data := struct {
			User        interface{}
			UserInitial string
			Errors      []string
			FormData    map[string]string
			CSRFToken   string
		}{
			User:        user,
			UserInitial: getUserInitial(user.Username),
			Errors:      errors,
			FormData: map[string]string{
				"app_name":        appName,
				"compose_content": composeContent,
			},
			CSRFToken: "",
		}
		
		tmpl := s.templates["app_create"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.ExecuteTemplate(w, "base", data)
		return
	}
	
	// GET request - show form
	data := struct {
		User        interface{}
		UserInitial string
		Errors      []string
		FormData    map[string]string
		CSRFToken   string
	}{
		User:        user,
		UserInitial: getUserInitial(user.Username),
		Errors:      nil,
		FormData:    map[string]string{},
		CSRFToken:   "",
	}
	
	tmpl := s.templates["app_create"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "base", data)
}

// isValidAppName validates app name format
func isValidAppName(appName string) bool {
	// Only allow letters, numbers, hyphens, and underscores
	pattern := `^[a-zA-Z0-9_-]+$`
	match, _ := regexp.MatchString(pattern, appName)
	return match && len(appName) <= 50
}

// validateComposeContent validates the docker-compose.yml content
func validateComposeContent(content string, appName string) error {
	// Parse YAML
	var data map[string]interface{}
	err := yaml.Unmarshal([]byte(content), &data)
	if err != nil {
		return fmt.Errorf("Invalid YAML format: %v", err)
	}
	
	// Check for services section
	services, ok := data["services"]
	if !ok {
		return fmt.Errorf("The YAML must contain a 'services' section")
	}
	
	servicesMap, ok := services.(map[string]interface{})
	if !ok || len(servicesMap) == 0 {
		return fmt.Errorf("The 'services' section must contain at least one service")
	}
	
	// Check that there's exactly one service
	if len(servicesMap) != 1 {
		return fmt.Errorf("Docker compose file must contain exactly one service")
	}
	
	// Get the service name
	var serviceName string
	for name := range servicesMap {
		serviceName = name
		break
	}
	
	// Check that service name matches app name
	if serviceName != appName {
		return fmt.Errorf("The service name ('%s') must match the App Name ('%s')", serviceName, appName)
	}
	
	// Check that service has an image
	service := servicesMap[serviceName].(map[string]interface{})
	if _, ok := service["image"]; !ok {
		return fmt.Errorf("The service '%s' must contain an 'image' key", serviceName)
	}
	
	return nil
}

// createAppScaffold creates the directory structure and docker-compose.yml for a new app
func createAppScaffold(appsDir, appName, composeContent string) error {
	appPath := filepath.Join(appsDir, appName)
	
	// Create app directory
	err := os.MkdirAll(appPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create app directory: %v", err)
	}
	
	// Create mnt directory
	mntPath := filepath.Join(appPath, "mnt")
	err = os.MkdirAll(mntPath, 0755)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(appPath)
		return fmt.Errorf("failed to create mnt directory: %v", err)
	}
	
	// Write docker-compose.yml
	composePath := filepath.Join(appPath, "docker-compose.yml")
	err = os.WriteFile(composePath, []byte(composeContent), 0644)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(appPath)
		return fmt.Errorf("failed to write docker-compose.yml: %v", err)
	}
	
	return nil
}