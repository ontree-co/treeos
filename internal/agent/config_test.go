package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemProvider_GetAll(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "agent-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use tempDir directly as the apps directory
	appsDir := tempDir

	// Test case 1: Valid configuration
	t.Run("ValidConfiguration", func(t *testing.T) {
		// Create nextcloud app directory
		nextcloudDir := filepath.Join(appsDir, "nextcloud")
		if err := os.MkdirAll(nextcloudDir, 0755); err != nil {
			t.Fatalf("Failed to create nextcloud directory: %v", err)
		}

		// Create valid app.yml
		validConfig := `id: "nextcloud"
name: "Nextcloud Suite"
primary_service: "app"
uptime_kuma_monitor: "nextcloud-web"
expected_services:
  - "app"
  - "db"
  - "redis"
`
		configPath := filepath.Join(nextcloudDir, "app.yml")
		if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Create plex app directory
		plexDir := filepath.Join(appsDir, "plex")
		if err := os.MkdirAll(plexDir, 0755); err != nil {
			t.Fatalf("Failed to create plex directory: %v", err)
		}

		// Create another valid config
		plexConfig := `id: "plex"
name: "Plex Media Server"
primary_service: "plex"
uptime_kuma_monitor: "plex-server"
expected_services:
  - "plex"
`
		plexConfigPath := filepath.Join(plexDir, "app.yml")
		if err := os.WriteFile(plexConfigPath, []byte(plexConfig), 0644); err != nil {
			t.Fatalf("Failed to write plex config file: %v", err)
		}

		// Test the provider
		provider := NewFilesystemProvider(tempDir)
		configs, err := provider.GetAll()
		if err != nil {
			t.Fatalf("GetAll() returned error: %v", err)
		}

		if len(configs) != 2 {
			t.Errorf("Expected 2 configs, got %d", len(configs))
		}

		// Verify the configs
		for _, config := range configs {
			if config.ID == "nextcloud" {
				if config.Name != "Nextcloud Suite" {
					t.Errorf("Expected name 'Nextcloud Suite', got '%s'", config.Name)
				}
				if config.PrimaryService != "app" {
					t.Errorf("Expected primary_service 'app', got '%s'", config.PrimaryService)
				}
				if len(config.ExpectedServices) != 3 {
					t.Errorf("Expected 3 services for nextcloud, got %d", len(config.ExpectedServices))
				}
			} else if config.ID == "plex" {
				if config.Name != "Plex Media Server" {
					t.Errorf("Expected name 'Plex Media Server', got '%s'", config.Name)
				}
				if config.PrimaryService != "plex" {
					t.Errorf("Expected primary_service 'plex', got '%s'", config.PrimaryService)
				}
				if len(config.ExpectedServices) != 1 {
					t.Errorf("Expected 1 service for plex, got %d", len(config.ExpectedServices))
				}
			} else {
				t.Errorf("Unexpected config ID: %s", config.ID)
			}
		}
	})

	// Test case 2: Malformed YAML
	t.Run("MalformedYAML", func(t *testing.T) {
		// Create a directory with malformed YAML
		badDir := filepath.Join(appsDir, "badapp")
		if err := os.MkdirAll(badDir, 0755); err != nil {
			t.Fatalf("Failed to create bad app directory: %v", err)
		}

		malformedConfig := `id: "badapp
name: Malformed App
primary_service app
expected_services
  - service1
`
		badConfigPath := filepath.Join(badDir, "app.yml")
		if err := os.WriteFile(badConfigPath, []byte(malformedConfig), 0644); err != nil {
			t.Fatalf("Failed to write malformed config file: %v", err)
		}

		provider := NewFilesystemProvider(tempDir)
		configs, err := provider.GetAll()

		// Should not return error, but should skip the malformed file
		if err != nil {
			t.Fatalf("GetAll() returned error: %v", err)
		}

		// Should still return the valid configs from previous test
		if len(configs) != 2 {
			t.Errorf("Expected 2 valid configs (malformed should be skipped), got %d", len(configs))
		}
	})

	// Test case 3: Missing required fields
	t.Run("MissingRequiredFields", func(t *testing.T) {
		// Create a directory with config missing required fields
		incompleteDir := filepath.Join(appsDir, "incomplete")
		if err := os.MkdirAll(incompleteDir, 0755); err != nil {
			t.Fatalf("Failed to create incomplete app directory: %v", err)
		}

		incompleteConfig := `id: "incomplete"
name: "Incomplete App"
# Missing primary_service and expected_services
`
		incompleteConfigPath := filepath.Join(incompleteDir, "app.yml")
		if err := os.WriteFile(incompleteConfigPath, []byte(incompleteConfig), 0644); err != nil {
			t.Fatalf("Failed to write incomplete config file: %v", err)
		}

		provider := NewFilesystemProvider(tempDir)
		configs, err := provider.GetAll()

		// Should not return error, but should skip the incomplete file
		if err != nil {
			t.Fatalf("GetAll() returned error: %v", err)
		}

		// Should still return only the valid configs
		if len(configs) != 2 {
			t.Errorf("Expected 2 valid configs (incomplete should be skipped), got %d", len(configs))
		}
	})

	// Test case 4: Non-existent root directory
	t.Run("NonExistentRootDirectory", func(t *testing.T) {
		provider := NewFilesystemProvider("/non/existent/path")
		_, err := provider.GetAll()

		if err == nil {
			t.Error("Expected error for non-existent root directory")
		}
	})

	// Test case 5: Empty apps directory
	t.Run("EmptyAppsDirectory", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "empty-test")
		if err != nil {
			t.Fatalf("Failed to create empty temp directory: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		provider := NewFilesystemProvider(emptyDir)
		configs, err := provider.GetAll()

		if err != nil {
			t.Fatalf("GetAll() returned error for empty apps directory: %v", err)
		}

		if len(configs) != 0 {
			t.Errorf("Expected 0 configs for empty apps directory, got %d", len(configs))
		}
	})

	// Test case 6: Directory without app.yml
	t.Run("DirectoryWithoutConfig", func(t *testing.T) {
		// Create a directory without config file
		noConfigDir := filepath.Join(appsDir, "noconfig")
		if err := os.MkdirAll(noConfigDir, 0755); err != nil {
			t.Fatalf("Failed to create no config directory: %v", err)
		}

		// Create some other file instead
		otherFile := filepath.Join(noConfigDir, "docker-compose.yml")
		if err := os.WriteFile(otherFile, []byte("version: '3'"), 0644); err != nil {
			t.Fatalf("Failed to write other file: %v", err)
		}

		provider := NewFilesystemProvider(tempDir)
		configs, err := provider.GetAll()

		if err != nil {
			t.Fatalf("GetAll() returned error: %v", err)
		}

		// Should still return only the valid configs (nextcloud and plex)
		if len(configs) != 2 {
			t.Errorf("Expected 2 valid configs (directory without config should be skipped), got %d", len(configs))
		}
	})
}

func TestParseConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "parse-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	provider := NewFilesystemProvider(tempDir)

	t.Run("ValidFile", func(t *testing.T) {
		validConfig := `id: "test"
name: "Test App"
primary_service: "main"
uptime_kuma_monitor: "test-monitor"
expected_services:
  - "main"
  - "worker"
`
		configPath := filepath.Join(tempDir, "valid.yaml")
		if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		config, err := provider.parseConfigFile(configPath)
		if err != nil {
			t.Fatalf("parseConfigFile() returned error: %v", err)
		}

		if config.ID != "test" {
			t.Errorf("Expected ID 'test', got '%s'", config.ID)
		}
		if config.Name != "Test App" {
			t.Errorf("Expected name 'Test App', got '%s'", config.Name)
		}
		if config.PrimaryService != "main" {
			t.Errorf("Expected primary_service 'main', got '%s'", config.PrimaryService)
		}
		if len(config.ExpectedServices) != 2 {
			t.Errorf("Expected 2 services, got %d", len(config.ExpectedServices))
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := provider.parseConfigFile("/non/existent/file.yaml")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		invalidConfig := `id: "test
name: Invalid YAML
`
		configPath := filepath.Join(tempDir, "invalid.yaml")
		if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err := provider.parseConfigFile(configPath)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})

	t.Run("MissingID", func(t *testing.T) {
		missingID := `name: "Test App"
primary_service: "main"
expected_services:
  - "main"
`
		configPath := filepath.Join(tempDir, "missing_id.yaml")
		if err := os.WriteFile(configPath, []byte(missingID), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err := provider.parseConfigFile(configPath)
		if err == nil {
			t.Error("Expected error for missing ID")
		}
	})

	t.Run("EmptyExpectedServices", func(t *testing.T) {
		emptyServices := `id: "test"
name: "Test App"
primary_service: "main"
expected_services: []
`
		configPath := filepath.Join(tempDir, "empty_services.yaml")
		if err := os.WriteFile(configPath, []byte(emptyServices), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err := provider.parseConfigFile(configPath)
		if err == nil {
			t.Error("Expected error for empty expected_services")
		}
	})
}

func TestScanForConfigs(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "scan-configs-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a complex directory structure
	dirs := []string{
		filepath.Join(tempDir, "apps", "app1"),
		filepath.Join(tempDir, "apps", "app2"),
		filepath.Join(tempDir, "apps", "app3", "subdir"),
		filepath.Join(tempDir, "other"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create app.yml files in various locations
	configFiles := []string{
		filepath.Join(tempDir, "apps", "app1", "app.yml"),
		filepath.Join(tempDir, "apps", "app2", "app.yml"),
		filepath.Join(tempDir, "apps", "app3", "app.yml"),
		// Not in the expected location, but should still be found
		filepath.Join(tempDir, "other", "app.yml"),
	}

	for _, file := range configFiles {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	// Create some other files that should not be found
	otherFiles := []string{
		filepath.Join(tempDir, "apps", "app1", "docker-compose.yml"),
		filepath.Join(tempDir, "apps", "app2", "config.yaml"),
		filepath.Join(tempDir, "README.md"),
	}

	for _, file := range otherFiles {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	// Test scanning
	foundConfigs, err := ScanForConfigs(tempDir)
	if err != nil {
		t.Fatalf("ScanForConfigs() returned error: %v", err)
	}

	if len(foundConfigs) != 4 {
		t.Errorf("Expected 4 config files, found %d", len(foundConfigs))
	}

	// Verify all expected files were found
	expectedMap := make(map[string]bool)
	for _, expected := range configFiles {
		expectedMap[expected] = false
	}

	for _, found := range foundConfigs {
		if _, ok := expectedMap[found]; ok {
			expectedMap[found] = true
		} else {
			t.Errorf("Found unexpected config file: %s", found)
		}
	}

	for file, found := range expectedMap {
		if !found {
			t.Errorf("Expected config file not found: %s", file)
		}
	}
}
