package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"treeos/internal/config"
	"treeos/internal/database"
	"treeos/internal/version"
)

func TestHandleSettings(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	// Initialize database
	if err := database.Initialize(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close() //nolint:errcheck,gosec // Test cleanup

	// Create test config
	cfg := &config.Config{
		AppsDir:      tmpDir + "/apps",
		DatabasePath: dbPath,
		ListenAddr:   ":3000",
	}

	// Create apps directory
	os.MkdirAll(cfg.AppsDir, 0755) //nolint:errcheck,gosec // Test setup

	// Create server instance
	s, err := New(cfg, version.Info{Version: "test"})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer s.Shutdown()

	// Create a test user
	user := &database.User{
		ID:          1,
		Username:    "testuser",
		IsSuperuser: true,
		IsActive:    true,
	}

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedInBody []string
	}{
		{
			name:           "GET settings page loads successfully",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedInBody: []string{
				"Settings",
				"LLM Configuration",
				// Note: Domain Configuration and Uptime Kuma Integration are hidden for initial release
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/settings", nil)
			ctx := context.WithValue(req.Context(), userContextKey, user)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			s.handleSettings(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			for _, expected := range tt.expectedInBody {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected body to contain %q, but it didn't. Body: %s", expected, body)
				}
			}
		})
	}
}

func TestHandleSettingsUpdate(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	// Initialize database
	if err := database.Initialize(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close() //nolint:errcheck,gosec // Test cleanup

	// Create test config
	cfg := &config.Config{
		AppsDir:      tmpDir + "/apps",
		DatabasePath: dbPath,
		ListenAddr:   ":3000",
	}

	// Create apps directory
	os.MkdirAll(cfg.AppsDir, 0755) //nolint:errcheck,gosec // Test setup

	// Create server instance
	s, err := New(cfg, version.Info{Version: "test"})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer s.Shutdown()

	// Create a test user
	user := &database.User{
		ID:          1,
		Username:    "testuser",
		IsSuperuser: true,
		IsActive:    true,
	}

	tests := []struct {
		name           string
		formData       url.Values
		expectedStatus int
		checkConfig    func(*testing.T, *Server)
	}{
		{
			name: "Update domain settings",
			formData: url.Values{
				"public_base_domain": {"example.com"},
				"tailscale_auth_key": {"tskey-auth-test123"},
				"tailscale_tags":     {"tag:ontree-apps"},
			},
			expectedStatus: http.StatusFound, // Redirect after save
			checkConfig: func(t *testing.T, s *Server) {
				if s.config.PublicBaseDomain != "example.com" {
					t.Errorf("Expected public domain to be example.com, got %s", s.config.PublicBaseDomain)
				}
				if s.config.TailscaleAuthKey != "tskey-auth-test123" {
					t.Errorf("Expected tailscale auth key to be set")
				}
				if s.config.TailscaleTags != "tag:ontree-apps" {
					t.Errorf("Expected tailscale tags to be tag:ontree-apps, got %s", s.config.TailscaleTags)
				}
			},
		},
		{
			name: "Update LLM settings",
			formData: url.Values{
				"agent_type":           {"cloud"},
				"agent_llm_api_key":    {"sk-test123"},
				"agent_llm_api_url":    {"https://api.openai.com/v1/chat/completions"},
				"agent_llm_model_cloud": {"gpt-4"},
			},
			expectedStatus: http.StatusFound,
			checkConfig: func(t *testing.T, s *Server) {
				if s.config.AgentLLMAPIKey != "sk-test123" {
					t.Errorf("Expected API key to be sk-test123, got %s", s.config.AgentLLMAPIKey)
				}
			},
		},
		{
			name: "Update Uptime Kuma settings",
			formData: url.Values{
				"uptime_kuma_base_url": {"http://localhost:3001"},
			},
			expectedStatus: http.StatusFound,
			checkConfig: func(t *testing.T, s *Server) {
				if s.config.UptimeKumaBaseURL != "http://localhost:3001" {
					t.Errorf("Expected Uptime Kuma URL to be http://localhost:3001, got %s", s.config.UptimeKumaBaseURL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/settings", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			ctx := context.WithValue(req.Context(), userContextKey, user)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			s.handleSettingsUpdate(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkConfig != nil {
				tt.checkConfig(t, s)
			}
		})
	}
}

func TestSettingsWithDatabaseMigration(t *testing.T) {
	// This test ensures that the settings page works even with an existing database
	// that doesn't have the new columns yet (migration test)

	// Create temp directory for test
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	// Initialize database first (simulating an existing database)
	if err := database.Initialize(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close() //nolint:errcheck,gosec // Test cleanup

	// Create test config
	cfg := &config.Config{
		AppsDir:      tmpDir + "/apps",
		DatabasePath: dbPath,
		ListenAddr:   ":3000",
	}

	// Create apps directory
	os.MkdirAll(cfg.AppsDir, 0755) //nolint:errcheck,gosec // Test setup

	// Setup test server
	s, err := New(cfg, version.Info{Version: "test"})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer s.Shutdown()

	// Create a test user
	user := &database.User{
		ID:          1,
		Username:    "testuser",
		IsSuperuser: true,
		IsActive:    true,
	}

	// Try to load settings page
	req := httptest.NewRequest("GET", "/settings", nil)
	req = req.WithContext(setUserContext(req.Context(), user))

	w := httptest.NewRecorder()
	s.handleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Check that the page loads with expected sections
	body := w.Body.String()
	expectedSections := []string{
		"LLM Configuration",
		// Note: Domain Configuration and Uptime Kuma Integration are hidden for initial release
	}

	for _, section := range expectedSections {
		if !strings.Contains(body, section) {
			t.Errorf("Expected settings page to contain section %q, but it didn't", section)
		}
	}
}
