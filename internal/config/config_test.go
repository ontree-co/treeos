package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func baseAppsDir() string {
	return filepath.Join(GetBasePath(), "apps")
}

func baseDBPath() string {
	return filepath.Join(GetBasePath(), "ontree.db")
}

func baseSharedPath() string {
	return filepath.Join(GetBasePath(), "shared")
}

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		wantRunMode    RunMode
		wantAppsDir    string
		wantDBPath     string
		wantListenAddr string
	}{
		{
			name:           "default production mode",
			envVars:        map[string]string{},
			wantRunMode:    ProductionMode,
			wantAppsDir:    baseAppsDir(),
			wantDBPath:     baseDBPath(),
			wantListenAddr: DefaultPort,
		},
		{
			name: "demo mode via environment",
			envVars: map[string]string{
				"TREEOS_RUN_MODE": "demo",
			},
			wantRunMode:    DemoMode,
			wantAppsDir:    "./apps",
			wantDBPath:     "./ontree.db",
			wantListenAddr: DefaultPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v) //nolint:errcheck,gosec // Test setup
			}

			// Get default config
			cfg := defaultConfig()

			expectedAppsDir := tt.wantAppsDir
			expectedDBPath := tt.wantDBPath
			if tt.wantRunMode == ProductionMode && (len(tt.envVars) == 0 || (len(tt.envVars) == 1 && tt.envVars["TREEOS_RUN_MODE"] == "production")) {
				expectedAppsDir = GetAppsPath()
				expectedDBPath = GetDatabasePath()
			}

			// Check results
			if cfg.RunMode != tt.wantRunMode {
				t.Errorf("RunMode = %v, want %v", cfg.RunMode, tt.wantRunMode)
			}
			if cfg.AppsDir != expectedAppsDir {
				t.Errorf("AppsDir = %v, want %v", cfg.AppsDir, expectedAppsDir)
			}
			if cfg.DatabasePath != expectedDBPath {
				t.Errorf("DatabasePath = %v, want %v", cfg.DatabasePath, expectedDBPath)
			}
			if cfg.ListenAddr != tt.wantListenAddr {
				t.Errorf("ListenAddr = %v, want %v", cfg.ListenAddr, tt.wantListenAddr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		wantRunMode RunMode
		wantAppsDir string
		wantDBPath  string
		wantListen  string
	}{
		{
			name: "demo mode overrides default paths",
			envVars: map[string]string{
				"TREEOS_RUN_MODE": "demo",
			},
			wantRunMode: DemoMode,
			wantAppsDir: "./apps",
			wantDBPath:  "./ontree.db",
			wantListen:  DefaultPort,
		},
		{
			name: "custom paths via environment",
			envVars: map[string]string{
				"ONTREE_APPS_DIR": "/custom/apps",
				"DATABASE_PATH":   "/custom/db.sqlite",
				"LISTEN_ADDR":     ":8080",
			},
			wantRunMode: ProductionMode,
			wantAppsDir: "/custom/apps",
			wantDBPath:  "/custom/db.sqlite",
			wantListen:  ":8080",
		},
		{
			name: "demo mode with custom listen address",
			envVars: map[string]string{
				"TREEOS_RUN_MODE": "demo",
				"LISTEN_ADDR":     ":4001",
			},
			wantRunMode: DemoMode,
			wantAppsDir: "./apps",
			wantDBPath:  "./ontree.db",
			wantListen:  ":4001",
		},
		{
			name: "production mode explicitly set",
			envVars: map[string]string{
				"TREEOS_RUN_MODE": "production",
			},
			wantRunMode: ProductionMode,
			wantAppsDir: baseAppsDir(),
			wantDBPath:  baseDBPath(),
			wantListen:  DefaultPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and clear environment
			origEnv := os.Environ()
			os.Clearenv()
			defer func() {
				// Restore environment
				os.Clearenv()
				for _, e := range origEnv {
					pair := strings.SplitN(e, "=", 2)
					if len(pair) == 2 {
						os.Setenv(pair[0], pair[1]) //nolint:errcheck,gosec // Test setup
					}
				}
			}()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v) //nolint:errcheck,gosec // Test setup
			}

			// Set a non-existent config path to avoid loading local .env
			os.Setenv("ONTREE_CONFIG_PATH", "/nonexistent/config.toml") //nolint:errcheck,gosec // Test setup

			// Load config
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			expectedAppsDir := tt.wantAppsDir
			expectedDBPath := tt.wantDBPath
			if tt.wantRunMode == ProductionMode && (len(tt.envVars) == 0 || (len(tt.envVars) == 1 && tt.envVars["TREEOS_RUN_MODE"] == "production")) {
				expectedAppsDir = GetAppsPath()
				expectedDBPath = GetDatabasePath()
			}

			// Check results
			if cfg.RunMode != tt.wantRunMode {
				t.Errorf("RunMode = %v, want %v", cfg.RunMode, tt.wantRunMode)
			}
			// For relative paths, check if they were converted to absolute paths correctly
			if strings.HasPrefix(expectedAppsDir, "./") {
				// For relative paths, just check that it ends with the expected suffix
				expectedSuffix := strings.TrimPrefix(expectedAppsDir, "./")
				if !strings.HasSuffix(cfg.AppsDir, expectedSuffix) {
					t.Errorf("AppsDir = %v, want path ending with %v", cfg.AppsDir, expectedSuffix)
				}
			} else if cfg.AppsDir != expectedAppsDir {
				t.Errorf("AppsDir = %v, want %v", cfg.AppsDir, expectedAppsDir)
			}
			if cfg.DatabasePath != expectedDBPath {
				t.Errorf("DatabasePath = %v, want %v", cfg.DatabasePath, expectedDBPath)
			}
			if cfg.ListenAddr != tt.wantListen {
				t.Errorf("ListenAddr = %v, want %v", cfg.ListenAddr, tt.wantListen)
			}
		})
	}
}

func TestGetSharedPath(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name:    "production mode returns base shared path",
			envVars: map[string]string{},
		},
		{
			name: "demo mode returns ./shared",
			envVars: map[string]string{
				"TREEOS_RUN_MODE": "demo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v) //nolint:errcheck,gosec // Test setup
			}

			// Get shared path
			got := GetSharedPath()

			expected := filepath.Join(GetBasePath(), "shared")
			if GetBasePath() == "." {
				expected = "./shared"
			}

			if got != expected {
				t.Errorf("GetSharedPath() = %v, want %v", got, expected)
			}
		})
	}
}

func TestGetSharedOllamaPath(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name:    "production mode returns base shared/ollama path",
			envVars: map[string]string{},
		},
		{
			name: "demo mode returns ./shared/ollama",
			envVars: map[string]string{
				"TREEOS_RUN_MODE": "demo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v) //nolint:errcheck,gosec // Test setup
			}

			// Get shared ollama path
			got := GetSharedOllamaPath()

			base := GetBasePath()
			expected := filepath.Join(base, "shared", "ollama")
			if base == "." {
				expected = "./shared/ollama"
			}

			if got != expected {
				t.Errorf("GetSharedOllamaPath() = %v, want %v", got, expected)
			}
		})
	}
}

func TestConfigMethods(t *testing.T) {
	t.Run("IsDemo", func(t *testing.T) {
		cfg := &Config{RunMode: DemoMode}
		if !cfg.IsDemo() {
			t.Error("IsDemo() = false for DemoMode, want true")
		}

		cfg.RunMode = ProductionMode
		if cfg.IsDemo() {
			t.Error("IsDemo() = true for ProductionMode, want false")
		}
	})

	t.Run("IsProduction", func(t *testing.T) {
		cfg := &Config{RunMode: ProductionMode}
		if !cfg.IsProduction() {
			t.Error("IsProduction() = false for ProductionMode, want true")
		}

		cfg.RunMode = DemoMode
		if cfg.IsProduction() {
			t.Error("IsProduction() = true for DemoMode, want false")
		}
	})

	t.Run("GetAppsDir", func(t *testing.T) {
		cfg := &Config{AppsDir: "/test/apps"}
		if got := cfg.GetAppsDir(); got != "/test/apps" {
			t.Errorf("GetAppsDir() = %v, want /test/apps", got)
		}
	})

	t.Run("String", func(t *testing.T) {
		cfg := &Config{
			RunMode:      DemoMode,
			AppsDir:      "./apps",
			DatabasePath: "./ontree.db",
			ListenAddr:   ":3000",
		}

		str := cfg.String()
		expectedParts := []string{
			"RunMode: demo",
			"AppsDir: ./apps",
			"DatabasePath: ./ontree.db",
			"ListenAddr: :3000",
		}

		for _, part := range expectedParts {
			if !contains(str, part) {
				t.Errorf("String() missing expected part: %s", part)
			}
		}
	})
}

func TestLoadWithConfigFile(t *testing.T) {
	// Save and restore environment
	origEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range origEnv {
			pair := strings.SplitN(e, "=", 2)
			if len(pair) == 2 {
				os.Setenv(pair[0], pair[1]) //nolint:errcheck,gosec // Test setup
			}
		}
	}()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "ontree.toml")

	configContent := `
apps_dir = "/config/apps"
database_path = "/config/ontree.db"
listen_addr = ":5000"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil { //nolint:gosec // Test file permissions //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set config path environment variable
	os.Clearenv()
	os.Setenv("ONTREE_CONFIG_PATH", configFile) //nolint:errcheck,gosec // Test setup

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify config values from file
	if cfg.AppsDir != "/config/apps" {
		t.Errorf("AppsDir = %v, want /config/apps", cfg.AppsDir)
	}
	if cfg.DatabasePath != "/config/ontree.db" {
		t.Errorf("DatabasePath = %v, want /config/ontree.db", cfg.DatabasePath)
	}
	if cfg.ListenAddr != ":5000" {
		t.Errorf("ListenAddr = %v, want :5000", cfg.ListenAddr)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || len(s) >= len(substr) && contains(s[1:], substr)
}
