// Package docker provides Docker client functionality for managing containerized applications
package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	"gopkg.in/yaml.v3"
)

// App represents a discovered application
type App struct {
	Name   string     `json:"name"`
	Path   string     `json:"path"`
	Status string     `json:"status"`
	Config *AppConfig `json:"config,omitempty"`
	Error  string     `json:"error,omitempty"`
	Emoji  string     `json:"emoji,omitempty"`
}

// AppConfig represents the application configuration from docker-compose.yml
type AppConfig struct {
	Container struct {
		Image string `json:"image"`
	} `json:"container"`
	Ports map[string]string `json:"ports,omitempty"`
}

// ComposeService represents a service definition in docker-compose.yml
type ComposeService struct {
	Image       string   `yaml:"image"`
	Ports       []string `yaml:"ports,omitempty"`
	Environment []string `yaml:"environment,omitempty"`
	Volumes     []string `yaml:"volumes,omitempty"`
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
		config, emoji, err := parseDockerCompose(composePath)
		if err != nil {
			app.Status = "error"
			app.Error = fmt.Sprintf("Failed to parse docker-compose.yml: %v", err)
		} else {
			app.Config = config
			app.Emoji = emoji
			// Get container status
			app.Status = c.getContainerStatus(app.Name)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// parseDockerCompose parses a docker-compose.yml file
func parseDockerCompose(path string) (*AppConfig, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	var compose Compose
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, "", err
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

	// Extract emoji from x-ontree metadata
	emoji := ""
	if compose.XOnTree != nil {
		emoji = compose.XOnTree.Emoji
	}

	return config, emoji, nil
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
	config, emoji, err := parseDockerCompose(composePath)
	if err != nil {
		app.Status = "error"
		app.Error = fmt.Sprintf("Failed to parse docker-compose.yml: %v", err)
	} else {
		app.Config = config
		app.Emoji = emoji
		app.Status = c.getContainerStatus(appName)
	}

	return app, nil
}

// StartApp starts or creates a Docker container for the specified application
func (c *Client) StartApp(appsDir, appName string) error {
	ctx := context.Background()
	containerName := fmt.Sprintf("ontree-%s", appName)

	// Get app details
	app, err := c.GetAppDetails(appsDir, appName)
	if err != nil {
		return err
	}

	// Check if container already exists
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var existingContainer *container.Summary
	for i := range containers {
		for _, name := range containers[i].Names {
			if strings.TrimPrefix(name, "/") == containerName {
				existingContainer = &containers[i]
				break
			}
		}
		if existingContainer != nil {
			break
		}
	}

	// If container exists but is stopped, start it
	if existingContainer != nil {
		if existingContainer.State != "running" {
			if err := c.dockerClient.ContainerStart(ctx, existingContainer.ID, container.StartOptions{}); err != nil {
				return fmt.Errorf("failed to start container: %w", err)
			}
		}
		return nil
	}

	// Parse docker-compose.yml to get full config
	composePath := filepath.Join(app.Path, "docker-compose.yml")
	composeData, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("failed to read docker-compose.yml: %w", err)
	}

	var compose Compose
	if err := yaml.Unmarshal(composeData, &compose); err != nil {
		return fmt.Errorf("failed to parse docker-compose.yml: %w", err)
	}

	// Get the first service
	var service ComposeService
	for _, svc := range compose.Services {
		service = svc
		break
	}

	// Create container config
	config := &container.Config{
		Image: service.Image,
		Env:   service.Environment,
	}

	// Create host config
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Add port bindings
	if len(service.Ports) > 0 {
		hostConfig.PortBindings = nat.PortMap{}
		exposedPorts := nat.PortSet{}

		for _, portMapping := range service.Ports {
			parts := strings.Split(portMapping, ":")
			if len(parts) == 2 {
				containerPort := nat.Port(fmt.Sprintf("%s/tcp", parts[1]))
				exposedPorts[containerPort] = struct{}{}
				hostConfig.PortBindings[containerPort] = []nat.PortBinding{
					{
						HostPort: parts[0],
					},
				}
			}
		}
		config.ExposedPorts = exposedPorts
	}

	// Add volume bindings
	if len(service.Volumes) > 0 {
		hostConfig.Binds = []string{}
		for _, volumeMapping := range service.Volumes {
			parts := strings.Split(volumeMapping, ":")
			if len(parts) == 2 {
				hostPath := parts[0]
				// Convert relative paths to absolute paths
				if strings.HasPrefix(hostPath, "./") {
					hostPath = filepath.Join(app.Path, hostPath[2:])
					// Create directory if it doesn't exist
					if err := os.MkdirAll(hostPath, 0750); err != nil {
						return fmt.Errorf("failed to create volume directory: %w", err)
					}
				}
				hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", hostPath, parts[1]))
			}
		}
	}

	// Pull the image first
	reader, err := c.dockerClient.ImagePull(ctx, service.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
			log.Printf("Failed to close reader: %v", err)
		}
	}()

	// Wait for pull to complete
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("failed to read pull response: %w", err)
	}

	// Create the container
	resp, err := c.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start the container
	if err := c.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// StopApp stops the Docker container for the specified application
func (c *Client) StopApp(appName string) error {
	ctx := context.Background()
	containerName := fmt.Sprintf("ontree-%s", appName)

	// Find the container
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var containerID string
	for _, cont := range containers {
		for _, name := range cont.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				containerID = cont.ID
				break
			}
		}
		if containerID != "" {
			break
		}
	}

	if containerID == "" {
		return fmt.Errorf("container not found: %s", containerName)
	}

	// Stop the container with 10 second timeout
	timeout := 10
	if err := c.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

// RecreateApp recreates the Docker container for the specified application
func (c *Client) RecreateApp(appsDir, appName string) error {
	// Stop the container if it's running
	if err := c.StopApp(appName); err != nil {
		// Ignore error if container doesn't exist - this is expected
		log.Printf("Note: failed to stop app %s (this is expected if container doesn't exist): %v", appName, err)
	}

	// Delete the container
	if err := c.DeleteAppContainer(appName); err != nil {
		// Ignore error if container doesn't exist - this is expected
		log.Printf("Note: failed to delete app container %s (this is expected if container doesn't exist): %v", appName, err)
	}

	// Start a new container
	return c.StartApp(appsDir, appName)
}

// DeleteAppContainer deletes the Docker container for the specified application
func (c *Client) DeleteAppContainer(appName string) error {
	ctx := context.Background()
	containerName := fmt.Sprintf("ontree-%s", appName)

	// Find the container
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var containerID string
	for _, cont := range containers {
		for _, name := range cont.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				containerID = cont.ID
				break
			}
		}
		if containerID != "" {
			break
		}
	}

	if containerID == "" {
		return fmt.Errorf("container not found: %s", containerName)
	}

	// Remove the container
	if err := c.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}
