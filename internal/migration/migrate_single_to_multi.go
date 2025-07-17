package migration

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v3"
	"ontree-node/internal/config"
)

// MigrateSingleToMultiServiceApps migrates legacy single-container apps to the new multi-service format
func MigrateSingleToMultiServiceApps(cfg *config.Config, appNames []string) error {
	ctx := context.Background()
	
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Get all containers
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Filter containers based on naming pattern
	legacyApps := make(map[string]container.Summary)
	for _, cont := range containers {
		for _, name := range cont.Names {
			// Remove leading slash
			name = strings.TrimPrefix(name, "/")
			
			// Check if it matches legacy pattern: ontree-{appName} (no service name)
			if strings.HasPrefix(name, "ontree-") && strings.Count(name, "-") == 1 {
				appName := strings.TrimPrefix(name, "ontree-")
				
				// If specific apps are requested, filter by them
				if len(appNames) > 0 && !contains(appNames, appName) {
					continue
				}
				
				legacyApps[appName] = cont
			}
		}
	}

	if len(legacyApps) == 0 {
		log.Println("No legacy single-container apps found to migrate")
		return nil
	}

	log.Printf("Found %d legacy apps to migrate", len(legacyApps))

	// Process each legacy app
	successCount := 0
	for appName, cont := range legacyApps {
		log.Printf("Migrating app: %s", appName)
		
		if err := migrateApp(ctx, dockerClient, cfg, appName, cont); err != nil {
			log.Printf("ERROR: Failed to migrate app %s: %v", appName, err)
			continue
		}
		
		successCount++
		log.Printf("✓ Successfully migrated app: %s", appName)
	}

	log.Printf("Migration complete: %d/%d apps migrated successfully", successCount, len(legacyApps))
	
	if successCount < len(legacyApps) {
		return fmt.Errorf("migration completed with errors: %d/%d apps migrated", successCount, len(legacyApps))
	}

	return nil
}

func migrateApp(ctx context.Context, dockerClient *client.Client, cfg *config.Config, appName string, cont container.Summary) error {
	// 1. Create app directory structure
	appDir := filepath.Join(cfg.AppsDir, appName)
	mountDir := filepath.Join(cfg.AppsDir, "mount", appName, "app") // Using "app" as the service name
	
	// Check if app directory already exists
	if _, err := os.Stat(appDir); err == nil {
		// Check if docker-compose.yml already exists
		composePath := filepath.Join(appDir, "docker-compose.yml")
		if _, err := os.Stat(composePath); err == nil {
			log.Printf("App directory and docker-compose.yml already exist for %s, skipping migration", appName)
			return nil
		}
	}

	// Create directories
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}
	
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	// 2. Inspect container to get full configuration
	containerInfo, err := dockerClient.ContainerInspect(ctx, cont.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// 3. Generate docker-compose.yml from container configuration
	compose := generateComposeFromContainer(appName, containerInfo)
	
	// 4. Write docker-compose.yml
	composePath := filepath.Join(appDir, "docker-compose.yml")
	composeData, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("failed to marshal compose data: %w", err)
	}
	
	if err := os.WriteFile(composePath, composeData, 0600); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	// 5. Generate .env file if container has environment variables
	if len(containerInfo.Config.Env) > 0 {
		envPath := filepath.Join(appDir, ".env")
		envContent := generateEnvFile(containerInfo.Config.Env)
		if envContent != "" {
			if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
				log.Printf("WARNING: Failed to write .env file: %v", err)
			}
		}
	}

	// 6. Rename the container to new naming convention
	newContainerName := fmt.Sprintf("ontree-%s-app-1", appName)
	
	// Stop container if running
	if containerInfo.State.Running {
		log.Printf("Stopping container %s", containerInfo.Name)
		timeout := 10
		if err := dockerClient.ContainerStop(ctx, cont.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			log.Printf("WARNING: Failed to stop container: %v", err)
		}
	}

	// Rename container
	log.Printf("Renaming container from %s to %s", strings.TrimPrefix(containerInfo.Name, "/"), newContainerName)
	if err := dockerClient.ContainerRename(ctx, cont.ID, newContainerName); err != nil {
		log.Printf("WARNING: Failed to rename container: %v", err)
		// Continue anyway - the container can still be managed
	}

	// Restart container if it was running
	if containerInfo.State.Running {
		log.Printf("Restarting container %s", newContainerName)
		if err := dockerClient.ContainerStart(ctx, cont.ID, container.StartOptions{}); err != nil {
			log.Printf("WARNING: Failed to restart container: %v", err)
		}
	}

	return nil
}

func generateComposeFromContainer(appName string, containerInfo container.InspectResponse) map[string]interface{} {
	// Build service configuration
	service := map[string]interface{}{
		"image": containerInfo.Config.Image,
	}

	// Add container name
	service["container_name"] = fmt.Sprintf("ontree-%s-app-1", appName)

	// Extract ports
	if len(containerInfo.HostConfig.PortBindings) > 0 {
		var ports []string
		for containerPort, bindings := range containerInfo.HostConfig.PortBindings {
			for _, binding := range bindings {
				if binding.HostPort != "" {
					ports = append(ports, fmt.Sprintf("%s:%s", binding.HostPort, containerPort.Port()))
				}
			}
		}
		if len(ports) > 0 {
			service["ports"] = ports
		}
	}

	// Extract environment variables (skip system ones)
	var envVars []string
	for _, env := range containerInfo.Config.Env {
		// Skip common system environment variables
		if strings.HasPrefix(env, "PATH=") || 
		   strings.HasPrefix(env, "HOME=") ||
		   strings.HasPrefix(env, "HOSTNAME=") {
			continue
		}
		envVars = append(envVars, env)
	}
	if len(envVars) > 0 {
		service["environment"] = envVars
	}

	// Extract volumes
	if len(containerInfo.HostConfig.Binds) > 0 {
		var volumes []string
		for _, bind := range containerInfo.HostConfig.Binds {
			// Update bind mounts to use new mount directory structure
			parts := strings.Split(bind, ":")
			if len(parts) >= 2 {
				hostPath := parts[0]
				containerPath := parts[1]
				
				// If it's a relative path or under the old app directory, update it
				if strings.HasPrefix(hostPath, "./") || strings.Contains(hostPath, "/apps/"+appName) {
					// Use the new mount directory
					hostPath = fmt.Sprintf("/opt/ontree/apps/mount/%s/app", appName)
				}
				
				volumes = append(volumes, fmt.Sprintf("%s:%s", hostPath, containerPath))
			}
		}
		if len(volumes) > 0 {
			service["volumes"] = volumes
		}
	}

	// Add restart policy
	if containerInfo.HostConfig.RestartPolicy.Name != "" {
		service["restart"] = containerInfo.HostConfig.RestartPolicy.Name
	}

	// Extract networks (if not default)
	if containerInfo.NetworkSettings != nil && len(containerInfo.NetworkSettings.Networks) > 0 {
		var networks []string
		for netName := range containerInfo.NetworkSettings.Networks {
			if netName != "bridge" && netName != "host" && netName != "none" {
				networks = append(networks, netName)
			}
		}
		if len(networks) > 0 {
			service["networks"] = networks
		}
	}

	// Build compose structure
	compose := map[string]interface{}{
		"version": "3.8",
		"services": map[string]interface{}{
			"app": service, // Using "app" as the default service name
		},
	}

	// Add creation timestamp as a comment
	compose["x-ontree"] = map[string]interface{}{
		"migrated_from": "single-container",
		"migration_date": time.Now().Format(time.RFC3339),
	}

	return compose
}

func generateEnvFile(envVars []string) string {
	var envLines []string
	
	for _, env := range envVars {
		// Skip system environment variables
		if strings.HasPrefix(env, "PATH=") || 
		   strings.HasPrefix(env, "HOME=") ||
		   strings.HasPrefix(env, "HOSTNAME=") {
			continue
		}
		
		// Check if it looks like a secret (contains KEY, SECRET, PASSWORD, TOKEN)
		upperEnv := strings.ToUpper(env)
		if strings.Contains(upperEnv, "KEY") || 
		   strings.Contains(upperEnv, "SECRET") || 
		   strings.Contains(upperEnv, "PASSWORD") || 
		   strings.Contains(upperEnv, "TOKEN") {
			// Add to .env file
			envLines = append(envLines, env)
		}
	}
	
	return strings.Join(envLines, "\n")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}