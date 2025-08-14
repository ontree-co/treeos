// Package config provides configuration management for the OnTree application.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration settings for the application
type Config struct {
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
	PublicBaseDomain    string `toml:"public_base_domain"`
	TailscaleBaseDomain string `toml:"tailscale_base_domain"`

	// Monitoring feature flag
	MonitoringEnabled bool `toml:"monitoring_enabled"`

	// Agent configuration
	AgentEnabled       bool   `toml:"agent_enabled"`
	AgentCheckInterval string `toml:"agent_check_interval"` // e.g., "5m", "1h"
	AgentLLMAPIKey     string `toml:"agent_llm_api_key"`
	AgentLLMAPIURL     string `toml:"agent_llm_api_url"`
	AgentLLMModel      string `toml:"agent_llm_model"`
	AgentConfigDir     string `toml:"agent_config_dir"`     // Directory with app.homeserver.yaml files
	UptimeKumaBaseURL  string `toml:"uptime_kuma_base_url"` // Base URL for Uptime Kuma API
}

// defaultConfig returns the default configuration based on the platform
func defaultConfig() *Config {
	config := &Config{
		DatabasePath:       "ontree.db",
		ListenAddr:         DefaultPort,
		PostHogHost:        "https://app.posthog.com",
		MonitoringEnabled:  true,  // Enabled by default
		AgentEnabled:       false, // Disabled by default until configured
		AgentCheckInterval: "5m",  // Default 5 minutes
		AgentConfigDir:     "/opt/homeserver-config",
	}

	// Platform-specific defaults for AppsDir
	switch runtime.GOOS {
	case "linux":
		config.AppsDir = "/opt/ontree/apps"
	case "darwin":
		config.AppsDir = "./apps"
	default:
		config.AppsDir = "./apps"
	}

	return config
}

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	// Start with default configuration
	config := defaultConfig()

	// Try to load from config.toml if it exists
	configPath := "config.toml"
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
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

	if tailscaleBaseDomain := os.Getenv("TAILSCALE_BASE_DOMAIN"); tailscaleBaseDomain != "" {
		config.TailscaleBaseDomain = tailscaleBaseDomain
	}

	if monitoringEnabled := os.Getenv("MONITORING_ENABLED"); monitoringEnabled != "" {
		config.MonitoringEnabled = monitoringEnabled == "true" || monitoringEnabled == "1"
	}

	// Agent environment variables
	if agentEnabled := os.Getenv("AGENT_ENABLED"); agentEnabled != "" {
		config.AgentEnabled = agentEnabled == "true" || agentEnabled == "1"
	}

	if agentCheckInterval := os.Getenv("AGENT_CHECK_INTERVAL"); agentCheckInterval != "" {
		config.AgentCheckInterval = agentCheckInterval
	}

	if agentLLMAPIKey := os.Getenv("AGENT_LLM_API_KEY"); agentLLMAPIKey != "" {
		config.AgentLLMAPIKey = agentLLMAPIKey
	}

	if agentLLMAPIURL := os.Getenv("AGENT_LLM_API_URL"); agentLLMAPIURL != "" {
		config.AgentLLMAPIURL = agentLLMAPIURL
	}

	if agentLLMModel := os.Getenv("AGENT_LLM_MODEL"); agentLLMModel != "" {
		config.AgentLLMModel = agentLLMModel
	}

	if agentConfigDir := os.Getenv("AGENT_CONFIG_DIR"); agentConfigDir != "" {
		config.AgentConfigDir = agentConfigDir
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
	parts = append(parts, fmt.Sprintf("AppsDir: %s", c.AppsDir))
	parts = append(parts, fmt.Sprintf("DatabasePath: %s", c.DatabasePath))
	parts = append(parts, fmt.Sprintf("ListenAddr: %s", c.ListenAddr))
	return strings.Join(parts, ", ")
}
