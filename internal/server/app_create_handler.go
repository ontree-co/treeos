// Package server provides HTTP server functionality for the OnTree application
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"treeos/internal/config"
	"treeos/internal/security"
	"treeos/internal/yamlutil"
	"treeos/pkg/compose"
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
		envContent := r.FormValue("env_content")
		emoji := r.FormValue("emoji")

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
			// Validate YAML syntax and structure
			if err := yamlutil.ValidateComposeFile(composeContent); err != nil {
				errors = append(errors, err.Error())
			}
		}

		if len(errors) == 0 && s.dockerClient != nil {
			// Create the application
			err := s.createAppScaffold(appName, composeContent, envContent, emoji)
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
			"env_content":     envContent,
			"emoji":           emoji,
		}
		data["CSRFToken"] = ""
		data["Emojis"] = getRandomEmojis(7)
		data["SelectedEmoji"] = emoji

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
	data["Emojis"] = getRandomEmojis(7)
	data["SelectedEmoji"] = ""

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

// createAppScaffold creates the directory structure and docker-compose.yml for a new app
func (s *Server) createAppScaffold(appName, composeContent, envContent, emoji string) error {
	appPath := filepath.Join(s.config.AppsDir, appName)

	// Create the app structure
	if err := s.createAppScaffoldInternal(appPath, appName, composeContent, envContent, emoji); err != nil {
		return err
	}

	// Generate app.yml without template flag
	if err := s.generateAppYaml(appPath, appName, composeContent); err != nil {
		log.Printf("Warning: Failed to generate app.yml for %s: %v", appName, err)
		// Continue anyway - agent can generate it later
	}

	// Automatically create and start containers if compose service is available
	if s.composeSvc != nil {
		// Start containers after creation
		err := s.startContainersForNewApp(appName, appPath, composeContent)
		if err != nil {
			log.Printf("Warning: Failed to start containers for app %s: %v", appName, err)
			// Don't fail app creation if containers can't be started
			// User can manually start them later
		}
	}

	return nil
}

// createAppScaffoldInternal creates the basic app structure without starting containers
func (s *Server) createAppScaffoldInternal(appPath, appName, composeContent, envContent, emoji string) error {

	// Create app directory
	err := os.MkdirAll(appPath, 0750)
	if err != nil {
		return fmt.Errorf("failed to create app directory: %v", err)
	}

	// Check if this app uses shared models directory
	if usesSharedModels(composeContent) {
		if err := ensureSharedModelsDirectory(); err != nil {
			log.Printf("Warning: Failed to setup shared models directory: %v", err)
			// Continue anyway - user can create it manually
		}
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

	// Always create .env file with Docker Compose naming configuration
	// First, add the naming configuration
	namingConfig := fmt.Sprintf("COMPOSE_PROJECT_NAME=ontree-%s\nCOMPOSE_SEPARATOR=-\n", strings.ToLower(appName))

	// If user provided env content, append it
	if envContent != "" {
		// Check if user's content already has COMPOSE_PROJECT_NAME (shouldn't override)
		if !strings.Contains(envContent, "COMPOSE_PROJECT_NAME=") {
			envContent = namingConfig + envContent
		}
	} else {
		envContent = namingConfig
	}

	envPath := filepath.Join(appPath, ".env")
	err = os.WriteFile(envPath, []byte(envContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to create .env file: %v", err)
	}

	// Extract host port from compose content
	hostPort, err := extractHostPort(composeContent)
	if err != nil {
		log.Printf("Warning: Could not extract host port from docker-compose: %v", err)
		// Continue anyway - port will be 0
	}

	// Add OnTree metadata to the compose file
	yamlData, err := yamlutil.ReadComposeWithMetadata(composePath)
	if err != nil {
		log.Printf("Warning: Failed to read compose file for metadata: %v", err)
		// Continue anyway - app is created on disk
	} else {
		// Set initial metadata
		metadata := &yamlutil.OnTreeMetadata{
			Subdomain: appName, // Default subdomain to app name
			HostPort:  hostPort,
			IsExposed: false,
			Emoji:     emoji,
		}
		yamlutil.SetOnTreeMetadata(yamlData, metadata)

		// Write back with metadata
		err = yamlutil.WriteComposeWithMetadata(composePath, yamlData)
		if err != nil {
			log.Printf("Warning: Failed to write compose metadata: %v", err)
			// Continue anyway - app is created without metadata
		}
	}

	return nil
}

// startContainersForNewApp starts containers for a newly created app
func (s *Server) startContainersForNewApp(appName, appPath, composeContent string) error {
	log.Printf("Starting containers for newly created app: %s at path: %s", appName, appPath)

	// Validate security rules first
	validator := security.NewValidator(appName)
	if err := validator.ValidateCompose([]byte(composeContent)); err != nil {
		log.Printf("Security validation failed for app %s: %v", appName, err)
		// Don't fail app creation, just skip container creation
		return fmt.Errorf("security validation failed: %v", err)
	}

	// Start the app using compose SDK
	ctx := context.Background()
	opts := compose.Options{
		WorkingDir: appPath,
	}

	// Check if .env file exists
	envFile := filepath.Join(appPath, ".env")
	if _, err := os.Stat(envFile); err == nil {
		opts.EnvFile = ".env" // Just the filename, not the full path
	}

	log.Printf("Calling compose.Up with WorkingDir: %s", opts.WorkingDir)

	// Create and start the compose project
	if err := s.composeSvc.Up(ctx, opts); err != nil {
		log.Printf("Failed to start containers for app %s: %v", appName, err)
		return fmt.Errorf("failed to start containers: %v", err)
	}

	log.Printf("Successfully started containers for app: %s", appName)
	return nil
}

// createAppScaffoldFromTemplate creates an app from a template with initial_setup_required flag
func (s *Server) createAppScaffoldFromTemplate(appName, composeContent, envContent, emoji string) error {
	appPath := filepath.Join(s.config.AppsDir, appName)

	// Create the app structure normally
	if err := s.createAppScaffoldInternal(appPath, appName, composeContent, envContent, emoji); err != nil {
		return err
	}

	// Generate app.yml with initial_setup_required flag
	if err := s.generateAppYamlWithFlags(appPath, appName, composeContent, true); err != nil {
		log.Printf("Warning: Failed to generate app.yml for %s: %v", appName, err)
		// Continue anyway - agent can generate it later
	}

	return nil
}

// generateAppYaml generates an app.yml file for the agent to use
func (s *Server) generateAppYaml(appPath, appName, composeContent string) error {
	return s.generateAppYamlWithFlags(appPath, appName, composeContent, false)
}

// generateAppYamlWithFlags generates an app.yml file with optional flags
func (s *Server) generateAppYamlWithFlags(appPath, appName, composeContent string, fromTemplate bool) error {
	// Parse docker-compose to extract services
	var compose map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &compose); err != nil {
		return fmt.Errorf("failed to parse docker-compose.yml: %w", err)
	}

	// Extract services - store only service names, not full container names
	services := []string{}
	primaryService := ""

	if servicesMap, ok := compose["services"].(map[string]interface{}); ok {
		for serviceName := range servicesMap {
			// Store just the service name
			services = append(services, serviceName)
			if primaryService == "" {
				// Use first service as primary by default
				primaryService = serviceName
			}
		}
	}

	// Create app.yml structure
	appConfig := map[string]interface{}{
		"id":                strings.ToLower(appName),
		"name":              strings.ToLower(appName),
		"primary_service":   primaryService,
		"expected_services": services,
	}

	// Add initial_setup_required flag if from template
	if fromTemplate {
		appConfig["initial_setup_required"] = true
	}

	// Add uptime_kuma_monitor field if it's an uptime-kuma app
	if strings.Contains(strings.ToLower(appName), "uptime") {
		appConfig["uptime_kuma_monitor"] = ""
	}

	// Marshal to YAML
	appYmlData, err := yaml.Marshal(appConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal app config: %w", err)
	}

	// Write app.yml file
	appYmlPath := filepath.Join(appPath, "app.yml")
	if err := os.WriteFile(appYmlPath, appYmlData, 0600); err != nil {
		return fmt.Errorf("failed to write app.yml: %w", err)
	}

	log.Printf("Successfully generated app.yml for %s", appName)
	return nil
}

// triggerAgentForApp sends a signal to the agent to process a specific app immediately
func (s *Server) triggerAgentForApp(appName string) {
	// TODO: Implement agent triggering mechanism
	// This could be done via:
	// 1. Database flag that agent checks
	// 2. Direct channel communication if agent is in same process
	// 3. HTTP API call to agent endpoint
	// For now, just log it
	log.Printf("Agent triggered for app %s - will process on next cycle", appName)
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

// usesSharedModels checks if the compose content references the shared models directory
func usesSharedModels(composeContent string) bool {
	// Check for both Linux and macOS paths
	return strings.Contains(composeContent, "/opt/ontree/sharedmodels") ||
		strings.Contains(composeContent, "./sharedmodels")
}

// ensureSharedModelsDirectory creates the shared models directory with proper permissions
func ensureSharedModelsDirectory() error {
	path := config.GetSharedModelsPath()
	
	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create directory
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
		
		// Set ownership to root:root (Ollama runs as root)
		if err := os.Chown(path, 0, 0); err != nil {
			return fmt.Errorf("failed to set ownership: %v", err)
		}
		
		log.Printf("Created shared models directory at %s with root:root ownership", path)
	}
	
	return nil
}
