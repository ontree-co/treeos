package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the configuration for a single application
// loaded from app.homeserver.yaml
type AppConfig struct {
	ID                string   `yaml:"id"`
	Name              string   `yaml:"name"`
	PrimaryService    string   `yaml:"primary_service"`
	UptimeKumaMonitor string   `yaml:"uptime_kuma_monitor"`
	ExpectedServices  []string `yaml:"expected_services"`
}

// ConfigProvider defines the interface for configuration providers
type ConfigProvider interface {
	GetAll() ([]AppConfig, error)
}

// FilesystemProvider implements ConfigProvider by reading from the filesystem
type FilesystemProvider struct {
	rootDir string
}

// NewFilesystemProvider creates a new FilesystemProvider
func NewFilesystemProvider(rootDir string) *FilesystemProvider {
	return &FilesystemProvider{
		rootDir: rootDir,
	}
}

// GetAll scans the root directory for all app.homeserver.yaml files
// and returns parsed AppConfig structs
func (fp *FilesystemProvider) GetAll() ([]AppConfig, error) {
	// Check if root directory exists
	if _, err := os.Stat(fp.rootDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("root directory does not exist: %s", fp.rootDir)
	}

	var configs []AppConfig

	// Walk through the apps directory
	appsDir := filepath.Join(fp.rootDir, "apps")
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		// If apps directory doesn't exist, return empty list
		return configs, nil
	}

	// Read all subdirectories in the apps directory
	entries, err := ioutil.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read apps directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Look for app.homeserver.yaml in each app directory
		configPath := filepath.Join(appsDir, entry.Name(), "app.homeserver.yaml")

		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Skip directories without app.homeserver.yaml
			continue
		}

		// Read and parse the config file
		config, err := fp.parseConfigFile(configPath)
		if err != nil {
			// Log error but continue processing other apps
			// In production, you might want to handle this differently
			fmt.Printf("Warning: Failed to parse config file %s: %v\n", configPath, err)
			continue
		}

		configs = append(configs, *config)
	}

	return configs, nil
}

// parseConfigFile reads and parses a single app.homeserver.yaml file
func (fp *FilesystemProvider) parseConfigFile(path string) (*AppConfig, error) {
	// Read the file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if config.ID == "" {
		return nil, fmt.Errorf("config missing required field: id")
	}
	if config.Name == "" {
		return nil, fmt.Errorf("config missing required field: name")
	}
	if config.PrimaryService == "" {
		return nil, fmt.Errorf("config missing required field: primary_service")
	}
	if len(config.ExpectedServices) == 0 {
		return nil, fmt.Errorf("config missing required field: expected_services")
	}

	return &config, nil
}

// ScanForConfigs is a utility function that scans a directory tree
// for all app.homeserver.yaml files (alternative implementation)
func ScanForConfigs(rootDir string) ([]string, error) {
	var configPaths []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for app.homeserver.yaml files
		if !info.IsDir() && info.Name() == "app.homeserver.yaml" {
			configPaths = append(configPaths, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan for config files: %w", err)
	}

	return configPaths, nil
}

