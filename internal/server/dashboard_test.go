package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"treeos/internal/config"
	"treeos/internal/database"
	"treeos/internal/docker"
	"treeos/internal/version"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleDashboard_DisplaysApps(t *testing.T) {
	// Skip if Docker is not available
	dockerClient, err := docker.NewClient()
	if err != nil {
		t.Skip("Docker not available, skipping test:", err)
	}
	defer dockerClient.Close()

	// Create a temporary apps directory for testing
	tempDir := t.TempDir()
	appsDir := filepath.Join(tempDir, "apps")

	// Create test app directories with docker-compose.yml files
	testApps := []string{"openwebui-amd", "uptime-kuma"}
	for _, appName := range testApps {
		appDir := filepath.Join(appsDir, appName)
		if err := os.MkdirAll(appDir, 0755); err != nil {
			t.Fatalf("Failed to create app directory %s: %v", appDir, err)
		}
		
		// Create a minimal docker-compose.yml
		composeContent := fmt.Sprintf(`version: '3.8'
services:
  %s:
    image: nginx:latest
    ports:
      - "8080:80"
`, appName)
		composeFile := filepath.Join(appDir, "docker-compose.yml")
		if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
			t.Fatalf("Failed to create docker-compose.yml for %s: %v", appName, err)
		}
	}

	// Create test config with the temporary apps directory
	cfg := &config.Config{
		AppsDir:           appsDir,
		DatabasePath:      ":memory:",
		ListenAddr:        ":3000",
		MonitoringEnabled: true,
	}

	// Create server instance
	s, err := New(cfg, version.Info{Version: "test"})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer s.Shutdown()

	// Create a test user in context
	testUser := &database.User{
		ID:          1,
		Username:    "testuser",
		IsSuperuser: true,
		IsActive:    true,
	}

	// Create request with user in context
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), userContextKey, testUser)
	req = req.WithContext(ctx)

	// Create response recorder
	w := httptest.NewRecorder()

	// Call handleDashboard
	s.handleDashboard(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check that the response contains expected apps
	body := w.Body.String()

	// Log the apps section for debugging
	if strings.Contains(body, "No applications found") {
		t.Error("Dashboard shows 'No applications found' but test apps were created")
	}

	// Check for specific app names that we created in the test
	expectedApps := testApps

	missingApps := []string{}
	for _, appName := range expectedApps {
		if !strings.Contains(body, appName) {
			missingApps = append(missingApps, appName)
		}
	}

	if len(missingApps) > 0 {
		t.Errorf("Dashboard does not display the following apps: %v", missingApps)

		// Log what the dashboard is actually showing
		startIdx := strings.Index(body, "Applications")
		if startIdx != -1 {
			endIdx := strings.Index(body[startIdx:], "</section>")
			if endIdx != -1 {
				t.Logf("Applications section content:\n%s", body[startIdx:startIdx+endIdx])
			}
		}
	}

	// Additional debugging: Check if dockerClient is properly scanning apps
	apps, err := s.dockerClient.ScanApps(cfg.AppsDir)
	if err != nil {
		t.Errorf("Failed to scan apps directly: %v", err)
	}
	t.Logf("Direct scan found %d apps", len(apps))
	for _, app := range apps {
		t.Logf("  - %s", app.Name)
	}
}
