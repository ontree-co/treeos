package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"treeos/internal/config"
)

func TestHandleAPIAppLogs(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a server without compose service to test basic routing
	s := &Server{
		config: &config.Config{
			AppsDir: tmpDir,
		},
		composeSvc: nil, // No compose service for basic tests
	}

	tests := []struct {
		name           string
		method         string
		appName        string
		queryParams    string
		setupApp       bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Method not allowed",
			method:         http.MethodPost,
			appName:        "test-app",
			setupApp:       true,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "Empty app name",
			method:         http.MethodGet,
			appName:        "",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "App name is required",
		},
		{
			name:           "Service unavailable (no compose service)",
			method:         http.MethodGet,
			appName:        "test-app",
			setupApp:       true,
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "Compose service not available",
		},
		// Note: App not found test is not possible when composeSvc is nil
		// because the compose service check happens first
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up app if needed
			if tt.setupApp && tt.appName != "" {
				appDir := filepath.Join(tmpDir, tt.appName)
				if err := os.MkdirAll(appDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create a docker-compose.yml file
				composeContent := `version: '3'
services:
  web:
    image: nginx:latest
`
				composeFile := filepath.Join(appDir, "docker-compose.yml")
				if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Create request
			url := "/api/apps/" + tt.appName + "/logs" + tt.queryParams
			req := httptest.NewRequest(tt.method, url, nil)
			rec := httptest.NewRecorder()

			// Handle request
			s.handleAPIAppLogs(rec, req)

			// Check status
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Check body
			if !bytes.Contains(rec.Body.Bytes(), []byte(tt.expectedBody)) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}

// Note: Testing headers would require a mock compose service
// because the service check happens before headers are set
