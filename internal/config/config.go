// Package config provides configuration management for the OnTree application.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// RunMode defines whether the application runs in demo or production mode
type RunMode string

const (
	// DemoMode runs with local directories and no special permissions
	DemoMode RunMode = "demo"
	// ProductionMode runs with /opt/ontree directories and service management
	ProductionMode RunMode = "production"
)

// Config holds all configuration settings for the application
type Config struct {
	// RunMode determines paths and behavior (demo or production)
	RunMode RunMode `toml:"run_mode"`

	// AppsDir is the directory where applications are stored
	AppsDir string `toml:"apps_dir"`

	// DatabasePath is the path to the SQLite database file
	DatabasePath string `toml:"database_path"`

	// ListenAddr is the address and port for the web server
	ListenAddr string `toml:"listen_addr"`

	// PostHog analytics configuration
	PostHogAPIKey string `toml:"posthog_api_key"`
	PostHogHost   string `toml:"posthog_host"`

	// Caddy integration configuration
	PublicBaseDomain string `toml:"public_base_domain"`

	// Tailscale integration configuration
	TailscaleAuthKey string `toml:"tailscale_auth_key"`
	TailscaleTags    string `toml:"tailscale_tags"` // e.g., "tag:ontree-apps"

	// Monitoring feature flag
	MonitoringEnabled bool `toml:"monitoring_enabled"`

	// Auto-update configuration
	AutoUpdateEnabled bool `toml:"auto_update_enabled"`

	// LLM configuration (for future features)
	AgentLLMAPIKey    string `toml:"agent_llm_api_key"`
	AgentLLMAPIURL    string `toml:"agent_llm_api_url"`
	AgentLLMModel     string `toml:"agent_llm_model"`
	UptimeKumaBaseURL string `toml:"uptime_kuma_base_url"` // Base URL for Uptime Kuma API
}

// GetSharedPath returns the base path for shared resources based on the run mode
func GetSharedPath() string {
	// Check for run mode environment variable
	if os.Getenv("TREEOS_RUN_MODE") == "demo" {
		return "./shared"
	}
	// Production mode
	return "/opt/ontree/shared"
}

// GetSharedOllamaPath returns the path to the shared Ollama models directory
func GetSharedOllamaPath() string {
	base := GetSharedPath()
	// Ensure relative paths maintain the ./ prefix for Docker
	if strings.HasPrefix(base, "./") {
		return base + "/ollama"
	}
	return filepath.Join(base, "ollama")
}

// defaultConfig returns the default configuration based on the run mode
func defaultConfig() *Config {
	// Determine run mode from environment or default to production
	runMode := ProductionMode
	if os.Getenv("TREEOS_RUN_MODE") == "demo" {
		runMode = DemoMode
	}

	config := &Config{
		RunMode:           runMode,
		ListenAddr:        DefaultPort,
		PostHogHost:       "https://app.posthog.com",
		MonitoringEnabled: true, // Enabled by default
		AutoUpdateEnabled: true,
	}

	// Set paths based on run mode
	if runMode == DemoMode {
		config.AppsDir = "./apps"
		config.DatabasePath = "./ontree.db"
	} else {
		// Production mode
		config.AppsDir = "/opt/ontree/apps"
		config.DatabasePath = "/opt/ontree/ontree.db"
	}

	return config
}

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	// Start with default configuration
	config := defaultConfig()

	// Try to load from config.toml if it exists
	configPath := os.Getenv("ONTREE_CONFIG_PATH")
	if configPath == "" {
		configPath = "config.toml"
	}
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
	}

	// Override run mode if set via environment
	if runMode := os.Getenv("TREEOS_RUN_MODE"); runMode != "" {
		if runMode == "demo" {
			config.RunMode = DemoMode
			// Update default paths for demo mode if not already set
			if config.AppsDir == "/opt/ontree/apps" {
				config.AppsDir = "./apps"
			}
			if config.DatabasePath == "/opt/ontree/ontree.db" {
				config.DatabasePath = "./ontree.db"
			}
		} else {
			config.RunMode = ProductionMode
		}
	}

	// Override with environment variables if set
	if appsDir := os.Getenv("ONTREE_APPS_DIR"); appsDir != "" {
		config.AppsDir = appsDir
	}

	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		config.DatabasePath = dbPath
	}

	if listenAddr := os.Getenv("LISTEN_ADDR"); listenAddr != "" {
		config.ListenAddr = listenAddr
	}

	if postHogAPIKey := os.Getenv("POSTHOG_API_KEY"); postHogAPIKey != "" {
		config.PostHogAPIKey = postHogAPIKey
	}

	if postHogHost := os.Getenv("POSTHOG_HOST"); postHogHost != "" {
		config.PostHogHost = postHogHost
	}

	if publicBaseDomain := os.Getenv("PUBLIC_BASE_DOMAIN"); publicBaseDomain != "" {
		config.PublicBaseDomain = publicBaseDomain
	}

	if tailscaleAuthKey := os.Getenv("TAILSCALE_AUTH_KEY"); tailscaleAuthKey != "" {
		config.TailscaleAuthKey = tailscaleAuthKey
	}

	if tailscaleTags := os.Getenv("TAILSCALE_TAGS"); tailscaleTags != "" {
		config.TailscaleTags = tailscaleTags
	}

	if monitoringEnabled := os.Getenv("MONITORING_ENABLED"); monitoringEnabled != "" {
		config.MonitoringEnabled = monitoringEnabled == "true" || monitoringEnabled == "1"
	}

	if autoUpdateEnabled := os.Getenv("AUTO_UPDATE_ENABLED"); autoUpdateEnabled != "" {
		config.AutoUpdateEnabled = autoUpdateEnabled == "true" || autoUpdateEnabled == "1"
	}

	// LLM environment variables
	if agentLLMAPIKey := os.Getenv("AGENT_LLM_API_KEY"); agentLLMAPIKey != "" {
		config.AgentLLMAPIKey = agentLLMAPIKey
	}

	if agentLLMAPIURL := os.Getenv("AGENT_LLM_API_URL"); agentLLMAPIURL != "" {
		config.AgentLLMAPIURL = agentLLMAPIURL
	}

	if agentLLMModel := os.Getenv("AGENT_LLM_MODEL"); agentLLMModel != "" {
		config.AgentLLMModel = agentLLMModel
	}

	if uptimeKumaBaseURL := os.Getenv("UPTIME_KUMA_BASE_URL"); uptimeKumaBaseURL != "" {
		config.UptimeKumaBaseURL = uptimeKumaBaseURL
	}

	// Ensure AppsDir is absolute
	if !filepath.IsAbs(config.AppsDir) {
		absPath, err := filepath.Abs(config.AppsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for apps_dir: %w", err)
		}
		config.AppsDir = absPath
	}

	return config, nil
}

// GetAppsDir returns the configured apps directory
func (c *Config) GetAppsDir() string {
	return c.AppsDir
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("RunMode: %s", c.RunMode))
	parts = append(parts, fmt.Sprintf("AppsDir: %s", c.AppsDir))
	parts = append(parts, fmt.Sprintf("DatabasePath: %s", c.DatabasePath))
	parts = append(parts, fmt.Sprintf("ListenAddr: %s", c.ListenAddr))
	return strings.Join(parts, ", ")
}

// IsDemo returns true if running in demo mode
func (c *Config) IsDemo() bool {
	return c.RunMode == DemoMode
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.RunMode == ProductionMode
}
