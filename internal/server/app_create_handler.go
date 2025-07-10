// Package server provides HTTP server functionality for the OnTree application
package server

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"ontree-node/internal/yamlutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
			err := s.createAppScaffold(appName, composeContent)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to create application: %v", err))
			} else {
				log.Printf("Successfully created application: %s", appName)

				// Set success message
				session, err := s.sessionStore.Get(r, "ontree-session")
				if err != nil {
					log.Printf("Failed to get session: %v", err)
					// Continue anyway - not critical
				}
				session.AddFlash(fmt.Sprintf("Application '%s' has been created successfully! You can now manage it from the app detail page.", appName), "success")
				if err := session.Save(r, w); err != nil {
					log.Printf("Failed to save session: %v", err)
				}

				// Redirect to app detail page
				http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusFound)
				return
			}
		}

		// Render with errors
		data := s.baseTemplateData(user)
		data["Errors"] = errors
		data["FormData"] = map[string]string{
			"app_name":        appName,
			"compose_content": composeContent,
		}
		data["CSRFToken"] = ""

		tmpl := s.templates["app_create"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// GET request - show form
	data := s.baseTemplateData(user)
	data["Errors"] = nil
	data["FormData"] = map[string]string{}
	data["CSRFToken"] = ""

	tmpl := s.templates["app_create"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// isValidAppName validates app name format
func isValidAppName(appName string) bool {
	// Only allow letters, numbers, hyphens, and underscores
	pattern := `^[a-zA-Z0-9_-]+$`
	match, err := regexp.MatchString(pattern, appName)
	if err != nil {
		log.Printf("Failed to match regex: %v", err)
		return false
	}
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
	service, ok := servicesMap[serviceName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Invalid service structure for '%s'", serviceName)
	}
	if _, ok := service["image"]; !ok {
		return fmt.Errorf("The service '%s' must contain an 'image' key", serviceName)
	}

	return nil
}

// createAppScaffold creates the directory structure and docker-compose.yml for a new app
func (s *Server) createAppScaffold(appName, composeContent string) error {
	appPath := filepath.Join(s.config.AppsDir, appName)

	// Create app directory
	err := os.MkdirAll(appPath, 0750)
	if err != nil {
		return fmt.Errorf("failed to create app directory: %v", err)
	}

	// Create mnt directory
	mntPath := filepath.Join(appPath, "mnt")
	err = os.MkdirAll(mntPath, 0750)
	if err != nil {
		// Clean up on failure
		if err := os.RemoveAll(appPath); err != nil {
			log.Printf("Failed to clean up app directory: %v", err)
		}
		return fmt.Errorf("failed to create mnt directory: %v", err)
	}

	// Write docker-compose.yml
	composePath := filepath.Join(appPath, "docker-compose.yml")
	err = os.WriteFile(composePath, []byte(composeContent), 0600)
	if err != nil {
		// Clean up on failure
		if err := os.RemoveAll(appPath); err != nil {
			log.Printf("Failed to clean up app directory: %v", err)
		}
		return fmt.Errorf("failed to write docker-compose.yml: %v", err)
	}

	// Extract host port from compose content
	hostPort, err := extractHostPort(composeContent)
	if err != nil {
		log.Printf("Warning: Could not extract host port from docker-compose: %v", err)
		// Continue anyway - port will be 0
	}

	// Add OnTree metadata to the compose file
	compose, err := yamlutil.ReadComposeWithMetadata(composePath)
	if err != nil {
		log.Printf("Warning: Failed to read compose file for metadata: %v", err)
		// Continue anyway - app is created on disk
	} else {
		// Set initial metadata
		metadata := &yamlutil.OnTreeMetadata{
			Subdomain: appName, // Default subdomain to app name
			HostPort:  hostPort,
			IsExposed: false,
		}
		yamlutil.SetOnTreeMetadata(compose, metadata)

		// Write back with metadata
		err = yamlutil.WriteComposeWithMetadata(composePath, compose)
		if err != nil {
			log.Printf("Warning: Failed to write compose metadata: %v", err)
			// Continue anyway - app is created without metadata
		}
	}

	return nil
}

// extractHostPort extracts the host port from a docker-compose.yml content
func extractHostPort(composeContent string) (int, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal([]byte(composeContent), &data)
	if err != nil {
		return 0, fmt.Errorf("failed to parse YAML: %w", err)
	}

	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("no services found in docker-compose")
	}

	// Get the first service
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

		// ports can be a slice of strings
		portsSlice, ok := ports.([]interface{})
		if !ok {
			continue
		}

		// Parse the first port mapping
		if len(portsSlice) > 0 {
			portStr, ok := portsSlice[0].(string)
			if !ok {
				continue
			}

			// Extract host port from format "8080:3000"
			parts := strings.Split(portStr, ":")
			if len(parts) >= 1 {
				// Parse the host port
				var hostPort int
				_, err := fmt.Sscanf(parts[0], "%d", &hostPort)
				if err == nil && hostPort > 0 {
					return hostPort, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("no host port found in docker-compose")
}
