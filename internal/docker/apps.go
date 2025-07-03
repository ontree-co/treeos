package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"gopkg.in/yaml.v3"
)

// App represents a discovered application
type App struct {
	Name   string      `json:"name"`
	Path   string      `json:"path"`
	Status string      `json:"status"`
	Config *AppConfig  `json:"config,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// AppConfig represents the application configuration from docker-compose.yml
type AppConfig struct {
	Container struct {
		Image string `json:"image"`
	} `json:"container"`
	Ports map[string]string `json:"ports,omitempty"`
}

// DockerComposeService represents a service in docker-compose.yml
type DockerComposeService struct {
	Image       string            `yaml:"image"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment []string          `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
}

// DockerCompose represents the docker-compose.yml structure
type DockerCompose struct {
	Version  string                          `yaml:"version"`
	Services map[string]DockerComposeService `yaml:"services"`
}

// ScanApps scans the apps directory for applications
func (c *Client) ScanApps(appsDir string) ([]*App, error) {
	var apps []*App
	
	// Check if apps directory exists
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		return apps, nil // Return empty list if directory doesn't exist
	}
	
	// Read directory entries
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read apps directory: %w", err)
	}
	
	// Process each subdirectory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		appPath := filepath.Join(appsDir, entry.Name())
		composePath := filepath.Join(appPath, "docker-compose.yml")
		
		// Check if docker-compose.yml exists
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			continue
		}
		
		// Create app entry
		app := &App{
			Name: entry.Name(),
			Path: appPath,
		}
		
		// Parse docker-compose.yml
		config, err := parseDockerCompose(composePath)
		if err != nil {
			app.Status = "error"
			app.Error = fmt.Sprintf("Failed to parse docker-compose.yml: %v", err)
		} else {
			app.Config = config
			// Get container status
			app.Status = c.getContainerStatus(app.Name)
		}
		
		apps = append(apps, app)
	}
	
	return apps, nil
}

// parseDockerCompose parses a docker-compose.yml file
func parseDockerCompose(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var compose DockerCompose
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}
	
	// Extract configuration from the first service
	config := &AppConfig{
		Ports: make(map[string]string),
	}
	
	for _, service := range compose.Services {
		config.Container.Image = service.Image
		
		// Parse ports
		for _, port := range service.Ports {
			parts := strings.Split(port, ":")
			if len(parts) == 2 {
				config.Ports[parts[0]] = parts[1]
			}
		}
		
		break // Use first service for now
	}
	
	return config, nil
}

// getContainerStatus gets the status of a container with the ontree- prefix
func (c *Client) getContainerStatus(appName string) string {
	ctx := context.Background()
	containerName := fmt.Sprintf("ontree-%s", appName)
	
	// List containers (including stopped ones)
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "error"
	}
	
	// Find container by name
	for _, cont := range containers {
		for _, name := range cont.Names {
			// Container names start with / in Docker API
			if strings.TrimPrefix(name, "/") == containerName {
				return strings.ToLower(cont.State)
			}
		}
	}
	
	return "not_created"
}

// GetAppDetails retrieves detailed information about a specific app
func (c *Client) GetAppDetails(appsDir, appName string) (*App, error) {
	appPath := filepath.Join(appsDir, appName)
	composePath := filepath.Join(appPath, "docker-compose.yml")
	
	// Check if app exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("application not found: %s", appName)
	}
	
	app := &App{
		Name: appName,
		Path: appPath,
	}
	
	// Parse docker-compose.yml
	config, err := parseDockerCompose(composePath)
	if err != nil {
		app.Status = "error"
		app.Error = fmt.Sprintf("Failed to parse docker-compose.yml: %v", err)
	} else {
		app.Config = config
		app.Status = c.getContainerStatus(appName)
	}
	
	return app, nil
}