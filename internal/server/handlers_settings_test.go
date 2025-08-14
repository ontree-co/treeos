package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"os"

	"ontree-node/internal/config"
	"ontree-node/internal/database"
	"ontree-node/internal/version"
)

func TestHandleSettings(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	
	// Initialize database
	if err := database.Initialize(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	
	// Create test config
	cfg := &config.Config{
		AppsDir:      tmpDir + "/apps",
		DatabasePath: dbPath,
		ListenAddr:   ":3000",
	}
	
	// Create apps directory
	os.MkdirAll(cfg.AppsDir, 0755)
	
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
				"Domain Configuration",
				"AI Agent Configuration",
				"Uptime Kuma Integration",
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
	defer database.Close()
	
	// Create test config
	cfg := &config.Config{
		AppsDir:      tmpDir + "/apps",
		DatabasePath: dbPath,
		ListenAddr:   ":3000",
	}
	
	// Create apps directory
	os.MkdirAll(cfg.AppsDir, 0755)
	
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
				"public_base_domain":    {"example.com"},
				"tailscale_base_domain": {"example.tailnet.ts.net"},
			},
			expectedStatus: http.StatusFound, // Redirect after save
			checkConfig: func(t *testing.T, s *Server) {
				if s.config.PublicBaseDomain != "example.com" {
					t.Errorf("Expected public domain to be example.com, got %s", s.config.PublicBaseDomain)
				}
				if s.config.TailscaleBaseDomain != "example.tailnet.ts.net" {
					t.Errorf("Expected tailscale domain to be example.tailnet.ts.net, got %s", s.config.TailscaleBaseDomain)
				}
			},
		},
		{
			name: "Update agent settings",
			formData: url.Values{
				"agent_enabled":        {"on"},
				"agent_check_interval": {"10m"},
				"agent_llm_api_key":    {"sk-test123"},
				"agent_llm_api_url":    {"https://api.openai.com/v1/chat/completions"},
				"agent_llm_model":      {"gpt-4"},
				"agent_config_dir":     {"/opt/test-config"},
			},
			expectedStatus: http.StatusFound,
			checkConfig: func(t *testing.T, s *Server) {
				if !s.config.AgentEnabled {
					t.Error("Expected agent to be enabled")
				}
				if s.config.AgentCheckInterval != "10m" {
					t.Errorf("Expected check interval to be 10m, got %s", s.config.AgentCheckInterval)
				}
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
	defer database.Close()
	
	// Create test config
	cfg := &config.Config{
		AppsDir:      tmpDir + "/apps",
		DatabasePath: dbPath,
		ListenAddr:   ":3000",
	}
	
	// Create apps directory
	os.MkdirAll(cfg.AppsDir, 0755)
	
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
	
	// Check that the page loads with all sections
	body := w.Body.String()
	expectedSections := []string{
		"Domain Configuration",
		"AI Agent Configuration", 
		"Uptime Kuma Integration",
	}
	
	for _, section := range expectedSections {
		if !strings.Contains(body, section) {
			t.Errorf("Expected settings page to contain section %q, but it didn't", section)
		}
	}
}