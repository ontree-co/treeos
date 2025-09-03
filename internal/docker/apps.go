// Package docker provides Docker client functionality for managing containerized applications
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
	Name     string                    `json:"name"`
	Path     string                    `json:"path"`
	Status   string                    `json:"status"`
	Services map[string]ComposeService `json:"services,omitempty"`
	Error    string                    `json:"error,omitempty"`
	Emoji    string                    `json:"emoji,omitempty"`
}

// ComposeService represents a service definition in docker-compose.yml
type ComposeService struct {
	Image       string   `json:"image" yaml:"image"`
	Ports       []string `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment []string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Volumes     []string `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}

// Compose represents the docker-compose.yml structure
type Compose struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
	XOnTree  *struct {
		Subdomain string `yaml:"subdomain,omitempty"`
		HostPort  int    `yaml:"host_port,omitempty"`
		IsExposed bool   `yaml:"is_exposed"`
		Emoji     string `yaml:"emoji,omitempty"`
	} `yaml:"x-ontree,omitempty"`
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
		services, emoji, err := parseDockerCompose(composePath)
		if err != nil {
			app.Status = "error"
			app.Error = fmt.Sprintf("Failed to parse docker-compose.yml: %v", err)
		} else {
			app.Services = services
			app.Emoji = emoji
			// Get container status
			app.Status = c.getContainerStatus(app.Name)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// parseDockerCompose parses a docker-compose.yml file and returns all services
func parseDockerCompose(path string) (map[string]ComposeService, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	var compose Compose
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, "", err
	}

	// Extract emoji from x-ontree metadata
	emoji := ""
	if compose.XOnTree != nil {
		emoji = compose.XOnTree.Emoji
	}

	// Return all services
	return compose.Services, emoji, nil
}

// getContainerStatus gets the status of containers for a compose app
func (c *Client) getContainerStatus(appName string) string {
	ctx := context.Background()

	// List containers (including stopped ones)
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "error"
	}

	// Look for containers with compose naming pattern: ontree-{appName}-{serviceName}-1
	// The appName here matches the directory name exactly (can be mixed case)
	// Convert to lowercase for the container prefix as per our naming convention
	appIdentifier := strings.ToLower(appName)
	prefix := fmt.Sprintf("ontree-%s-", appIdentifier)
	var runningCount, stoppedCount int

	for _, cont := range containers {
		for _, name := range cont.Names {
			// Container names start with / in Docker API
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, prefix) {
				if strings.ToLower(cont.State) == "running" {
					runningCount++
				} else {
					stoppedCount++
				}
			}
		}
	}

	// Determine aggregate status
	if runningCount > 0 && stoppedCount > 0 {
		return "partial"
	} else if runningCount > 0 {
		return "running"
	} else if stoppedCount > 0 {
		return "exited"
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
	services, emoji, err := parseDockerCompose(composePath)
	if err != nil {
		app.Status = "error"
		app.Error = fmt.Sprintf("Failed to parse docker-compose.yml: %v", err)
	} else {
		app.Services = services
		app.Emoji = emoji
		app.Status = c.getContainerStatus(appName)
	}

	return app, nil
}
