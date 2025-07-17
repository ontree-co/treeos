package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ontree-node/internal/config"
	"ontree-node/pkg/compose"
)

func TestCreateApp(t *testing.T) {
	// Create temporary directory for tests
	tempDir := t.TempDir()
	
	// Create test server
	cfg := &config.Config{
		AppsDir:      tempDir,
		DatabasePath: filepath.Join(tempDir, "test.db"),
		ListenAddr:   ":8080",
	}
	
	s := &Server{
		config: cfg,
	}

	tests := []struct {
		name           string
		request        CreateAppRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Valid app creation",
			request: CreateAppRequest{
				Name: "test-app",
				ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"`,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Valid app with env file",
			request: CreateAppRequest{
				Name: "test-app-env",
				ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx:latest`,
				EnvContent: "TEST_VAR=value\nANOTHER_VAR=value2",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Empty app name",
			request: CreateAppRequest{
				Name:        "",
				ComposeYAML: "version: '3.8'\nservices:\n  web:\n    image: nginx",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "App name is required",
		},
		{
			name: "Invalid app name",
			request: CreateAppRequest{
				Name:        "Test-App",
				ComposeYAML: "version: '3.8'\nservices:\n  web:\n    image: nginx",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid app name",
		},
		{
			name: "Empty YAML",
			request: CreateAppRequest{
				Name:        "test-app2",
				ComposeYAML: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Docker Compose YAML is required",
		},
		{
			name: "Invalid YAML",
			request: CreateAppRequest{
				Name:        "test-app3",
				ComposeYAML: "invalid: yaml: content:",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid YAML",
		},
		{
			name: "YAML without services",
			request: CreateAppRequest{
				Name:        "test-app4",
				ComposeYAML: "version: '3.8'",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "must contain a 'services' section",
		},
		{
			name: "Duplicate app",
			request: CreateAppRequest{
				Name: "duplicate-app",
				ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx`,
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal request to JSON
			reqBody, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			req := httptest.NewRequest("POST", "/api/apps", bytes.NewReader(reqBody))
			w := httptest.NewRecorder()

			// Handle request
			s.handleCreateApp(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" {
				body := w.Body.String()
				if !bytes.Contains([]byte(body), []byte(tt.expectedError)) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, body)
				}
			}

			// If successful, verify files were created
			if tt.expectedStatus == http.StatusCreated {
				appDir := filepath.Join(tempDir, tt.request.Name)
				composeFile := filepath.Join(appDir, "docker-compose.yml")
				
				// Check docker-compose.yml exists
				if _, err := os.Stat(composeFile); os.IsNotExist(err) {
					t.Errorf("docker-compose.yml was not created")
				}

				// Check content matches
				content, err := os.ReadFile(composeFile)
				if err != nil {
					t.Errorf("Failed to read docker-compose.yml: %v", err)
				}
				if string(content) != tt.request.ComposeYAML {
					t.Errorf("docker-compose.yml content doesn't match")
				}

				// Check .env file if provided
				if tt.request.EnvContent != "" {
					envFile := filepath.Join(appDir, ".env")
					if _, err := os.Stat(envFile); os.IsNotExist(err) {
						t.Errorf(".env file was not created")
					}
					
					envContent, err := os.ReadFile(envFile)
					if err != nil {
						t.Errorf("Failed to read .env file: %v", err)
					}
					if string(envContent) != tt.request.EnvContent {
						t.Errorf(".env content doesn't match")
					}
				}

				// Check mount directory
				mountDir := filepath.Join(tempDir, "mount", tt.request.Name)
				if _, err := os.Stat(mountDir); os.IsNotExist(err) {
					t.Errorf("Mount directory was not created")
				}
			}
		})
	}

	// Test duplicate app creation
	t.Run("Duplicate app error", func(t *testing.T) {
		req := CreateAppRequest{
			Name: "duplicate-app",
			ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx`,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/api/apps", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		s.handleCreateApp(w, httpReq)

		if w.Code != http.StatusConflict {
			t.Errorf("Expected status %d for duplicate app, got %d", http.StatusConflict, w.Code)
		}

		if !bytes.Contains(w.Body.Bytes(), []byte("already exists")) {
			t.Errorf("Expected 'already exists' error, got: %s", w.Body.String())
		}
	})
}

func TestUpdateApp(t *testing.T) {
	// Create temporary directory for tests
	tempDir := t.TempDir()
	
	// Create test server
	cfg := &config.Config{
		AppsDir:      tempDir,
		DatabasePath: filepath.Join(tempDir, "test.db"),
		ListenAddr:   ":8080",
	}
	
	s := &Server{
		config: cfg,
	}

	// Create an existing app for update tests
	existingAppName := "existing-app"
	existingAppDir := filepath.Join(tempDir, existingAppName)
	os.MkdirAll(existingAppDir, 0755)
	originalCompose := `version: '3.8'
services:
  web:
    image: nginx:1.19`
	os.WriteFile(filepath.Join(existingAppDir, "docker-compose.yml"), []byte(originalCompose), 0644)
	os.WriteFile(filepath.Join(existingAppDir, ".env"), []byte("OLD_VAR=old"), 0644)

	tests := []struct {
		name           string
		appName        string
		request        UpdateAppRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name:    "Valid update",
			appName: existingAppName,
			request: UpdateAppRequest{
				ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx:latest
  db:
    image: postgres:13`,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "Update with new env",
			appName: existingAppName,
			request: UpdateAppRequest{
				ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx:latest`,
				EnvContent: "NEW_VAR=new\nANOTHER_VAR=value",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "Non-existent app",
			appName: "non-existent",
			request: UpdateAppRequest{
				ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx`,
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "not found",
		},
		{
			name:    "Empty YAML",
			appName: existingAppName,
			request: UpdateAppRequest{
				ComposeYAML: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Docker Compose YAML is required",
		},
		{
			name:    "Invalid YAML",
			appName: existingAppName,
			request: UpdateAppRequest{
				ComposeYAML: "invalid: yaml: content:",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal request to JSON
			reqBody, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			req := httptest.NewRequest("PUT", "/api/apps/"+tt.appName, bytes.NewReader(reqBody))
			w := httptest.NewRecorder()

			// Handle request
			s.handleUpdateApp(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" {
				body := w.Body.String()
				if !bytes.Contains([]byte(body), []byte(tt.expectedError)) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, body)
				}
			}

			// If successful, verify files were updated
			if tt.expectedStatus == http.StatusOK {
				appDir := filepath.Join(tempDir, tt.appName)
				composeFile := filepath.Join(appDir, "docker-compose.yml")
				
				// Check docker-compose.yml was updated
				content, err := os.ReadFile(composeFile)
				if err != nil {
					t.Errorf("Failed to read docker-compose.yml: %v", err)
				}
				if string(content) != tt.request.ComposeYAML {
					t.Errorf("docker-compose.yml content doesn't match expected")
				}

				// Check .env file handling
				envFile := filepath.Join(appDir, ".env")
				if tt.request.EnvContent != "" {
					// Should exist with new content
					envContent, err := os.ReadFile(envFile)
					if err != nil {
						t.Errorf("Failed to read .env file: %v", err)
					}
					if string(envContent) != tt.request.EnvContent {
						t.Errorf(".env content doesn't match expected")
					}
				}
			}
		})
	}

	// Test removing .env file
	t.Run("Remove env file when empty", func(t *testing.T) {
		// First ensure .env exists
		envFile := filepath.Join(existingAppDir, ".env")
		os.WriteFile(envFile, []byte("TEST=value"), 0644)

		req := UpdateAppRequest{
			ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx`,
			EnvContent: "", // Empty env content should remove the file
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("PUT", "/api/apps/"+existingAppName, bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		s.handleUpdateApp(w, httpReq)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check that .env file was removed
		if _, err := os.Stat(envFile); !os.IsNotExist(err) {
			t.Errorf(".env file should have been removed")
		}
	})
}

func TestHTTPMethods(t *testing.T) {
	// Create test server
	s := &Server{
		config: &config.Config{
			AppsDir: t.TempDir(),
		},
	}

	t.Run("Create endpoint - wrong method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/apps", nil)
		w := httptest.NewRecorder()
		s.handleCreateApp(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})

	t.Run("Update endpoint - wrong method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/apps/test-app", nil)
		w := httptest.NewRecorder()
		s.handleUpdateApp(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

func TestJSONResponse(t *testing.T) {
	// Create temporary directory for tests
	tempDir := t.TempDir()
	
	// Create test server
	s := &Server{
		config: &config.Config{
			AppsDir: tempDir,
		},
	}

	t.Run("Create success response", func(t *testing.T) {
		req := CreateAppRequest{
			Name: "json-test",
			ComposeYAML: `version: '3.8'
services:
  web:
    image: nginx`,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/api/apps", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		s.handleCreateApp(w, httpReq)

		if w.Code != http.StatusCreated {
			t.Fatalf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		// Check Content-Type
		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}

		// Parse response
		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}

		// Check response fields
		if success, ok := response["success"].(bool); !ok || !success {
			t.Errorf("Expected success=true in response")
		}

		if message, ok := response["message"].(string); !ok || message == "" {
			t.Errorf("Expected non-empty message in response")
		}

		if app, ok := response["app"].(map[string]interface{}); !ok {
			t.Errorf("Expected app object in response")
		} else {
			if name, ok := app["name"].(string); !ok || name != "json-test" {
				t.Errorf("Expected app name 'json-test', got '%v'", name)
			}
			if path, ok := app["path"].(string); !ok || path == "" {
				t.Errorf("Expected non-empty app path")
			}
		}
	})
}

func TestHandleAPIAppStart(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a mock server with compose service
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		composeSvc: &compose.Service{}, // This would be mocked in a real test
	}

	tests := []struct {
		name           string
		appName        string
		method         string
		setupFiles     func()
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Method not allowed",
			appName:        "test-app",
			method:         "GET",
			setupFiles:     func() {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			name:           "App not found",
			appName:        "non-existent",
			method:         "POST",
			setupFiles:     func() {},
			expectedStatus: http.StatusNotFound,
			expectedError:  "App 'non-existent' not found",
		},
		{
			name:    "Security validation fails - privileged mode",
			appName: "privileged-app",
			method:  "POST",
			setupFiles: func() {
				appDir := filepath.Join(tmpDir, "privileged-app")
				os.MkdirAll(appDir, 0755)
				composeContent := `version: '3.8'
services:
  web:
    image: nginx
    privileged: true`
				os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Security validation failed",
		},
		{
			name:    "Security validation fails - dangerous capabilities",
			appName: "dangerous-cap-app",
			method:  "POST",
			setupFiles: func() {
				appDir := filepath.Join(tmpDir, "dangerous-cap-app")
				os.MkdirAll(appDir, 0755)
				composeContent := `version: '3.8'
services:
  web:
    image: nginx
    cap_add:
      - SYS_ADMIN`
				os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Security validation failed",
		},
		{
			name:    "Security validation fails - invalid bind mount",
			appName: "invalid-mount-app",
			method:  "POST",
			setupFiles: func() {
				appDir := filepath.Join(tmpDir, "invalid-mount-app")
				os.MkdirAll(appDir, 0755)
				composeContent := `version: '3.8'
services:
  web:
    image: nginx
    volumes:
      - /etc/passwd:/etc/passwd`
				os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Security validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup files
			tt.setupFiles()

			// Create request
			req := httptest.NewRequest(tt.method, fmt.Sprintf("/api/apps/%s/start", tt.appName), nil)
			w := httptest.NewRecorder()

			// Handle request
			s.handleAPIAppStart(w, req)

			// Check status
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" && !strings.Contains(w.Body.String(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, w.Body.String())
			}
		})
	}
}

func TestHandleAPIAppStartSuccess(t *testing.T) {
	// This test would require mocking the compose service
	// For now, we'll just verify the validation passes for a valid compose file
	tmpDir := t.TempDir()
	
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		// In a real test, we would mock the compose service to verify it's called correctly
		composeSvc: nil, // Set to nil to get service unavailable error
	}

	// Create a valid app
	appName := "valid-app"
	appDir := filepath.Join(tmpDir, appName)
	os.MkdirAll(appDir, 0755)
	
	// Create mount directory as required by security rules
	mountDir := filepath.Join(tmpDir, "mount", appName, "web")
	os.MkdirAll(mountDir, 0755)
	
	composeContent := fmt.Sprintf(`version: '3.8'
services:
  web:
    image: nginx
    volumes:
      - %s:/var/www/html`, mountDir)
	
	os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644)

	// Create request
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/apps/%s/start", appName), nil)
	w := httptest.NewRecorder()

	// Handle request
	s.handleAPIAppStart(w, req)

	// Since compose service is nil, we expect service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
	
	if !strings.Contains(w.Body.String(), "Compose service not available") {
		t.Errorf("Expected 'Compose service not available', got '%s'", w.Body.String())
	}
}
func TestHandleAPIAppStop(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a mock server with compose service
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		composeSvc: &compose.Service{}, // This would be mocked in a real test
	}

	tests := []struct {
		name           string
		appName        string
		method         string
		setupFiles     func()
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Method not allowed",
			appName:        "test-app",
			method:         "GET",
			setupFiles:     func() {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			name:           "Empty app name",
			appName:        "",
			method:         "POST",
			setupFiles:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "App name is required",
		},
		{
			name:           "App not found",
			appName:        "non-existent",
			method:         "POST",
			setupFiles:     func() {},
			expectedStatus: http.StatusNotFound,
			expectedError:  "App 'non-existent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup files
			tt.setupFiles()

			// Create request
			req := httptest.NewRequest(tt.method, fmt.Sprintf("/api/apps/%s/stop", tt.appName), nil)
			w := httptest.NewRecorder()

			// Handle request
			s.handleAPIAppStop(w, req)

			// Check status
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" && !strings.Contains(w.Body.String(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, w.Body.String())
			}
		})
	}
}

func TestHandleAPIAppStopSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		// In a real test, we would mock the compose service to verify it's called correctly
		composeSvc: nil, // Set to nil to get service unavailable error
	}

	// Create a valid app
	appName := "valid-app"
	appDir := filepath.Join(tmpDir, appName)
	os.MkdirAll(appDir, 0755)
	
	composeContent := `version: '3.8'
services:
  web:
    image: nginx`
	
	os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644)

	// Create request
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/apps/%s/stop", appName), nil)
	w := httptest.NewRecorder()

	// Handle request
	s.handleAPIAppStop(w, req)

	// Since compose service is nil, we expect service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
	
	if !strings.Contains(w.Body.String(), "Compose service not available") {
		t.Errorf("Expected 'Compose service not available', got '%s'", w.Body.String())
	}
}

func TestHandleAPIAppDelete(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a mock server with compose service
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		composeSvc: &compose.Service{}, // This would be mocked in a real test
	}

	tests := []struct {
		name           string
		appName        string
		method         string
		setupFiles     func()
		expectedStatus int
		expectedError  string
		checkCleanup   bool
	}{
		{
			name:           "Method not allowed",
			appName:        "test-app",
			method:         "GET",
			setupFiles:     func() {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			name:           "Empty app name",
			appName:        "",
			method:         "DELETE",
			setupFiles:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "App name is required",
		},
		{
			name:           "App not found",
			appName:        "non-existent",
			method:         "DELETE",
			setupFiles:     func() {},
			expectedStatus: http.StatusNotFound,
			expectedError:  "App 'non-existent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup files
			tt.setupFiles()

			// Create request
			req := httptest.NewRequest(tt.method, fmt.Sprintf("/api/apps/%s", tt.appName), nil)
			w := httptest.NewRecorder()

			// Handle request
			s.handleAPIAppDelete(w, req)

			// Check status
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" && !strings.Contains(w.Body.String(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, w.Body.String())
			}
		})
	}
}

func TestHandleAPIAppDeleteSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		// In a real test, we would mock the compose service to verify it's called correctly
		composeSvc: nil, // Set to nil to get service unavailable error
	}

	// Create a valid app with directories
	appName := "valid-app"
	appDir := filepath.Join(tmpDir, appName)
	mountDir := filepath.Join(tmpDir, "mount", appName)
	
	os.MkdirAll(appDir, 0755)
	os.MkdirAll(mountDir, 0755)
	
	composeContent := `version: '3.8'
services:
  web:
    image: nginx`
	
	os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeContent), 0644)

	// Create request
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/apps/%s", appName), nil)
	w := httptest.NewRecorder()

	// Handle request
	s.handleAPIAppDelete(w, req)

	// Since compose service is nil, we expect service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
	
	if !strings.Contains(w.Body.String(), "Compose service not available") {
		t.Errorf("Expected 'Compose service not available', got '%s'", w.Body.String())
	}
}