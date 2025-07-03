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
}

// defaultConfig returns the default configuration based on the platform
func defaultConfig() *Config {
	config := &Config{
		DatabasePath: "ontree.db",
		ListenAddr:   ":8081",
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