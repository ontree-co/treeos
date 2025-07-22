package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"ontree-node/internal/yamlutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// handleMultiServiceAppCreate handles the creation of multi-service apps via UI
func (s *Server) handleMultiServiceAppCreate(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user")

	if r.Method == http.MethodGet {
		// Show the creation form
		data := map[string]interface{}{
			"User":         user,
			"Title":        "Create Multi-Service App",
			"FormData":     map[string]string{},
			"Errors":       []string{},
			"RandomEmojis": getRandomEmojis(7),
		}

		tmpl := s.templates["multiservice_create"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Handle POST request
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	appName := strings.TrimSpace(r.FormValue("app_name"))
	composeContent := r.FormValue("compose_content")
	envContent := r.FormValue("env_content")
	emoji := r.FormValue("emoji")

	// Collect validation errors
	var errors []string

	// Validate app name
	if appName == "" {
		errors = append(errors, "App name is required")
	} else if !appNameRegex.MatchString(appName) {
		errors = append(errors, "Invalid app name. Only lowercase letters, numbers, and hyphens are allowed")
	}

	// Validate YAML content
	if composeContent == "" {
		errors = append(errors, "Docker Compose YAML is required")
	} else {
		// Use centralized validation
		if err := yamlutil.ValidateComposeFile(composeContent); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate emoji if provided
	if emoji != "" && !yamlutil.IsValidEmoji(emoji) {
		errors = append(errors, "Invalid emoji selected")
		emoji = "" // Clear invalid emoji
	}

	// Check if app already exists
	if appName != "" {
		appPath := filepath.Join(s.config.AppsDir, appName)
		if _, err := os.Stat(appPath); err == nil {
			errors = append(errors, fmt.Sprintf("An app named '%s' already exists", appName))
		}
	}

	// If there are errors, show the form again with errors
	if len(errors) > 0 {
		data := map[string]interface{}{
			"User":  user,
			"Title": "Create Multi-Service App",
			"FormData": map[string]string{
				"app_name":        appName,
				"compose_content": composeContent,
				"env_content":     envContent,
				"emoji":           emoji,
			},
			"Errors":       errors,
			"RandomEmojis": getRandomEmojis(7),
		}

		tmpl := s.templates["multiservice_create"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Create the app using the API
	createReq := CreateAppRequest{
		Name:        appName,
		ComposeYAML: composeContent,
		EnvContent:  envContent,
	}

	// Add emoji to the compose file if provided
	if emoji != "" {
		// Parse the YAML to add emoji metadata
		var composeData map[string]interface{}
		if err := yaml.Unmarshal([]byte(composeContent), &composeData); err == nil {
			// Add x-ontree metadata
			if composeData["x-ontree"] == nil {
				composeData["x-ontree"] = make(map[string]interface{})
			}
			if ontreeData, ok := composeData["x-ontree"].(map[string]interface{}); ok {
				ontreeData["emoji"] = emoji
			}

			// Marshal back to YAML
			updatedYAML, err := yaml.Marshal(composeData)
			if err == nil {
				createReq.ComposeYAML = string(updatedYAML)
			}
		}
	}

	// Marshal the request to JSON
	reqBody, err := json.Marshal(createReq)
	if err != nil {
		log.Printf("Failed to marshal create request: %v", err)
		errors = append(errors, "Failed to process request")
		// Show form with errors
		data := map[string]interface{}{
			"User":  user,
			"Title": "Create Multi-Service App",
			"FormData": map[string]string{
				"app_name":        appName,
				"compose_content": composeContent,
				"env_content":     envContent,
				"emoji":           emoji,
			},
			"Errors":       errors,
			"RandomEmojis": getRandomEmojis(7),
		}

		tmpl := s.templates["multiservice_create"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Create a new request to the API endpoint
	apiReq, err := http.NewRequest(http.MethodPost, "/api/apps", strings.NewReader(string(reqBody)))
	if err != nil {
		log.Printf("Failed to create API request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	apiReq.Header.Set("Content-Type", "application/json")

	// Use a response recorder to capture the API response
	rr := &responseRecorder{
		header: make(http.Header),
		body:   &strings.Builder{},
	}

	// Call the API handler directly
	s.handleCreateApp(rr, apiReq.WithContext(r.Context()))

	// Check if the API call was successful
	if rr.status >= 200 && rr.status < 300 {
		// Success - set flash message and redirect
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		} else {
			session.AddFlash(fmt.Sprintf("Multi-service app '%s' created successfully!", appName), "success")
			if err := session.Save(r, w); err != nil {
				log.Printf("Failed to save session: %v", err)
			}
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusSeeOther)
		return
	}

	// API call failed - extract error message
	var apiError string
	apiError = rr.body.String()
	if apiError == "" {
		apiError = fmt.Sprintf("Failed to create app (status: %d)", rr.status)
	}

	errors = append(errors, apiError)

	// Show form with errors
	data := map[string]interface{}{
		"User":  user,
		"Title": "Create Multi-Service App",
		"FormData": map[string]string{
			"app_name":        appName,
			"compose_content": composeContent,
			"env_content":     envContent,
			"emoji":           emoji,
		},
		"Errors":       errors,
		"RandomEmojis": getRandomEmojis(7),
	}

	tmpl := s.templates["multiservice_create"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleMultiServiceAppEdit handles editing of multi-service apps via UI
func (s *Server) handleMultiServiceAppEdit(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user")

	// Extract app name from URL
	path := strings.TrimPrefix(r.URL.Path, "/apps/")
	appName := strings.TrimSuffix(path, "/edit-multiservice")

	if appName == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// Fetch app configuration using the API
		apiReq, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/apps/%s", appName), nil)
		if err != nil {
			log.Printf("Failed to create API request: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Use a response recorder to capture the API response
		rr := &responseRecorder{
			header: make(http.Header),
			body:   &strings.Builder{},
		}

		// Call the API handler directly
		s.handleGetApp(rr, apiReq.WithContext(r.Context()))

		if rr.status != http.StatusOK {
			if rr.status == http.StatusNotFound {
				http.Error(w, fmt.Sprintf("App '%s' not found", appName), http.StatusNotFound)
			} else {
				http.Error(w, "Failed to fetch app configuration", http.StatusInternalServerError)
			}
			return
		}

		// Parse the API response
		var apiResp struct {
			Success bool `json:"success"`
			App     struct {
				Name        string `json:"name"`
				ComposeYAML string `json:"compose_yaml"`
				EnvContent  string `json:"env_content"`
			} `json:"app"`
		}

		if err := json.Unmarshal([]byte(rr.body.String()), &apiResp); err != nil {
			log.Printf("Failed to parse API response: %v", err)
			http.Error(w, "Failed to parse app configuration", http.StatusInternalServerError)
			return
		}

		// Extract emoji from compose YAML if present
		var emoji string
		var composeData map[string]interface{}
		if err := yaml.Unmarshal([]byte(apiResp.App.ComposeYAML), &composeData); err == nil {
			if ontreeData, ok := composeData["x-ontree"].(map[string]interface{}); ok {
				if e, ok := ontreeData["emoji"].(string); ok {
					emoji = e
				}
			}
		}

		// Show the edit form
		data := map[string]interface{}{
			"User":    user,
			"Title":   fmt.Sprintf("Edit Multi-Service App: %s", appName),
			"AppName": appName,
			"FormData": map[string]string{
				"compose_content": apiResp.App.ComposeYAML,
				"env_content":     apiResp.App.EnvContent,
				"emoji":           emoji,
			},
			"Errors":       []string{},
			"RandomEmojis": getRandomEmojis(7),
		}

		tmpl := s.templates["multiservice_edit"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Handle POST request
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	composeContent := r.FormValue("compose_content")
	envContent := r.FormValue("env_content")
	emoji := r.FormValue("emoji")

	// Collect validation errors
	var errors []string

	// Validate YAML content
	if composeContent == "" {
		errors = append(errors, "Docker Compose YAML is required")
	} else {
		// Use centralized validation first
		if err := yamlutil.ValidateComposeFile(composeContent); err != nil {
			errors = append(errors, err.Error())
		}
		
		// Parse YAML for emoji processing
		var composeData map[string]interface{}
		if err := yaml.Unmarshal([]byte(composeContent), &composeData); err == nil {

			// Add emoji to the compose file if provided
			if emoji != "" {
				if !yamlutil.IsValidEmoji(emoji) {
					errors = append(errors, "Invalid emoji selected")
					emoji = "" // Clear invalid emoji
				} else {
					// Add x-ontree metadata
					if composeData["x-ontree"] == nil {
						composeData["x-ontree"] = make(map[string]interface{})
					}
					if ontreeData, ok := composeData["x-ontree"].(map[string]interface{}); ok {
						ontreeData["emoji"] = emoji
					}

					// Marshal back to YAML
					updatedYAML, err := yaml.Marshal(composeData)
					if err == nil {
						composeContent = string(updatedYAML)
					}
				}
			}
		}
	}

	// If there are errors, show the form again with errors
	if len(errors) > 0 {
		data := map[string]interface{}{
			"User":    user,
			"Title":   fmt.Sprintf("Edit Multi-Service App: %s", appName),
			"AppName": appName,
			"FormData": map[string]string{
				"compose_content": composeContent,
				"env_content":     envContent,
				"emoji":           emoji,
			},
			"Errors":       errors,
			"RandomEmojis": getRandomEmojis(7),
		}

		tmpl := s.templates["multiservice_edit"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Update the app using the API
	updateReq := UpdateAppRequest{
		ComposeYAML: composeContent,
		EnvContent:  envContent,
	}

	// Marshal the request to JSON
	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		log.Printf("Failed to marshal update request: %v", err)
		errors = append(errors, "Failed to process request")
		// Show form with errors
		data := map[string]interface{}{
			"User":    user,
			"Title":   fmt.Sprintf("Edit Multi-Service App: %s", appName),
			"AppName": appName,
			"FormData": map[string]string{
				"compose_content": composeContent,
				"env_content":     envContent,
				"emoji":           emoji,
			},
			"Errors":       errors,
			"RandomEmojis": getRandomEmojis(7),
		}

		tmpl := s.templates["multiservice_edit"]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Create a new request to the API endpoint
	apiReq, err := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/apps/%s", appName), strings.NewReader(string(reqBody)))
	if err != nil {
		log.Printf("Failed to create API request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	apiReq.Header.Set("Content-Type", "application/json")

	// Use a response recorder to capture the API response
	rr := &responseRecorder{
		header: make(http.Header),
		body:   &strings.Builder{},
	}

	// Call the API handler directly
	s.handleUpdateApp(rr, apiReq.WithContext(r.Context()))

	// Check if the API call was successful
	if rr.status >= 200 && rr.status < 300 {
		// Success - set flash message and redirect
		session, err := s.sessionStore.Get(r, "ontree-session")
		if err != nil {
			log.Printf("Failed to get session: %v", err)
		} else {
			session.AddFlash(fmt.Sprintf("Multi-service app '%s' updated successfully!", appName), "success")
			if err := session.Save(r, w); err != nil {
				log.Printf("Failed to save session: %v", err)
			}
		}
		http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusSeeOther)
		return
	}

	// API call failed - extract error message
	var apiError string
	apiError = rr.body.String()
	if apiError == "" {
		apiError = fmt.Sprintf("Failed to update app (status: %d)", rr.status)
	}

	errors = append(errors, apiError)

	// Show form with errors
	data := map[string]interface{}{
		"User":    user,
		"Title":   fmt.Sprintf("Edit Multi-Service App: %s", appName),
		"AppName": appName,
		"FormData": map[string]string{
			"compose_content": composeContent,
			"env_content":     envContent,
			"emoji":           emoji,
		},
		"Errors":       errors,
		"RandomEmojis": getRandomEmojis(7),
	}

	tmpl := s.templates["multiservice_edit"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// responseRecorder is a simple implementation to capture HTTP responses
type responseRecorder struct {
	status int
	header http.Header
	body   *strings.Builder
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

// Implement the Flusher interface
func (r *responseRecorder) Flush() {
	// No-op for our simple recorder
}
