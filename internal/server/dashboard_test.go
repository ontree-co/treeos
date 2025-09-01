package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"ontree-node/internal/config"
	"ontree-node/internal/database"
	"ontree-node/internal/docker"
	"ontree-node/internal/version"
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

	// Create test config with the real apps directory
	cfg := &config.Config{
		AppsDir:           "/opt/ontree/apps",
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
		t.Error("Dashboard shows 'No applications found' but apps exist in /opt/ontree/apps")
	}

	// Check for specific app names that we know exist
	expectedApps := []string{
		"openwebui-amd",
		"owui-amd-tuesday",
		"testnginx",
		"uptime-kuma",
	}

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
