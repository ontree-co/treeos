package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v3"
)

// UpdateAppConfigWithActualContainers updates app.yml with actual container names from Docker
func UpdateAppConfigWithActualContainers(appDir string) error {
	appName := filepath.Base(appDir)
	appYmlPath := filepath.Join(appDir, "app.yml")

	// Read existing app.yml
	data, err := os.ReadFile(appYmlPath)
	if err != nil {
		return fmt.Errorf("failed to read app.yml: %w", err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse app.yml: %w", err)
	}

	// Connect to Docker
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// List all containers
	containers, err := dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Find containers that belong to this app
	var actualServices []string
	var primaryService string

	for _, container := range containers {
		containerName := strings.TrimPrefix(container.Names[0], "/")

		// Check if container belongs to this app
		if strings.HasPrefix(containerName, appName+"-") ||
			strings.HasPrefix(containerName, appName+"_") ||
			containerName == appName {
			actualServices = append(actualServices, containerName)
			if primaryService == "" {
				primaryService = containerName
			}
		}
	}

	// Special cases for apps with non-standard naming
	if appName == "openwebui-amd" && len(actualServices) == 0 {
		// Check for open-webui and ollama-amd
		for _, container := range containers {
			containerName := strings.TrimPrefix(container.Names[0], "/")
			if containerName == "open-webui" || containerName == "ollama-amd" {
				actualServices = append(actualServices, containerName)
				if containerName == "open-webui" && primaryService == "" {
					primaryService = containerName
				}
			}
		}
	}

	if appName == "owui-amd-tuesday" && len(actualServices) == 0 {
		// This app might use similar naming to openwebui-amd
		for _, container := range containers {
			containerName := strings.TrimPrefix(container.Names[0], "/")
			if strings.Contains(containerName, "owui") || strings.Contains(containerName, "tuesday") {
				actualServices = append(actualServices, containerName)
				if primaryService == "" {
					primaryService = containerName
				}
			}
		}
	}

	// Update config if we found actual containers
	if len(actualServices) > 0 {
		config.ExpectedServices = actualServices
		config.PrimaryService = primaryService
	}

	// Write updated app.yml
	updatedData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	if err := os.WriteFile(appYmlPath, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write updated app.yml: %w", err)
	}

	fmt.Printf("Updated %s with %d actual services\n", appName, len(actualServices))
	return nil
}

// UpdateAllAppConfigsWithActualContainers updates all app.yml files with actual container names
func UpdateAllAppConfigsWithActualContainers(appsDir string) error {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("failed to read apps directory: %w", err)
	}

	updated := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appDir := filepath.Join(appsDir, entry.Name())
		appYmlPath := filepath.Join(appDir, "app.yml")

		// Check if app.yml exists
		if _, err := os.Stat(appYmlPath); os.IsNotExist(err) {
			continue
		}

		// Update with actual container names
		if err := UpdateAppConfigWithActualContainers(appDir); err != nil {
			fmt.Printf("Failed to update %s: %v\n", entry.Name(), err)
			continue
		}

		updated++
	}

	fmt.Printf("Updated %d app.yml files with actual container names\n", updated)
	return nil
}
