package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// InitialSetupHandler handles the initial setup for apps created from templates
type InitialSetupHandler struct {
	appsDir string
}

// NewInitialSetupHandler creates a new initial setup handler
func NewInitialSetupHandler(appsDir string) *InitialSetupHandler {
	return &InitialSetupHandler{
		appsDir: appsDir,
	}
}

// SetupProgress represents the progress of initial setup
type SetupProgress struct {
	Step       int
	TotalSteps int
	StepName   string
	Message    string
	Details    string
	IsError    bool
}

// HandleInitialSetup performs the initial setup for an app
func (h *InitialSetupHandler) HandleInitialSetup(ctx context.Context, config AppConfig, progressChan chan<- SetupProgress) error {
	appPath := filepath.Join(h.appsDir, config.Name)

	// Send initial progress
	h.sendProgress(progressChan, 1, 6, "Detecting image versions",
		fmt.Sprintf("Starting initial setup for %s...", config.Name), "")

	// Step 1: Read docker-compose.yml
	composePath := filepath.Join(appPath, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath)
	if err != nil {
		h.sendError(progressChan, 1, 6, "Failed to read docker-compose.yml", err.Error())
		return fmt.Errorf("failed to read docker-compose.yml: %w", err)
	}

	// Step 2: Parse and find images
	h.sendProgress(progressChan, 2, 6, "Fetching latest version information",
		"Analyzing Docker images in configuration...", "")

	images, err := h.extractImages(composeContent)
	if err != nil {
		h.sendError(progressChan, 2, 6, "Failed to parse configuration", err.Error())
		return fmt.Errorf("failed to extract images: %w", err)
	}

	// Step 3: Update images to latest versions
	h.sendProgress(progressChan, 3, 6, "Updating configuration",
		"Locking images to specific versions...", "")

	updatedContent, versionMap, err := h.updateImagesToLatestVersions(string(composeContent), images)
	if err != nil {
		h.sendError(progressChan, 3, 6, "Failed to update versions", err.Error())
		return fmt.Errorf("failed to update versions: %w", err)
	}

	// Show what versions were locked
	details := "Version locks applied:\n"
	for orig, newVer := range versionMap {
		details += fmt.Sprintf("  %s → %s\n", orig, newVer)
	}
	h.sendProgress(progressChan, 3, 6, "Updating configuration",
		"Configuration updated with version locks", details)

	// Write updated docker-compose.yml
	if err := os.WriteFile(composePath, []byte(updatedContent), 0600); err != nil {
		h.sendError(progressChan, 3, 6, "Failed to save configuration", err.Error())
		return fmt.Errorf("failed to write updated compose file: %w", err)
	}

	// Step 4: Pull Docker images
	h.sendProgress(progressChan, 4, 6, "Pulling Docker images",
		"Downloading latest Docker images...", "This may take several minutes")

	if err := h.pullImages(ctx, appPath, progressChan); err != nil {
		h.sendError(progressChan, 4, 6, "Failed to pull images", err.Error())
		return fmt.Errorf("failed to pull images: %w", err)
	}

	// Step 5: Start containers
	h.sendProgress(progressChan, 5, 6, "Starting containers",
		"Creating and starting Docker containers...", "")

	if err := h.startContainers(ctx, appPath, progressChan); err != nil {
		h.sendError(progressChan, 5, 6, "Failed to start containers", err.Error())
		return fmt.Errorf("failed to start containers: %w", err)
	}

	// Step 6: Remove initial_setup_required flag
	h.sendProgress(progressChan, 6, 6, "Finalizing setup",
		"Updating application configuration...", "")

	if err := h.removeSetupFlag(appPath); err != nil {
		h.sendError(progressChan, 6, 6, "Failed to finalize setup", err.Error())
		return fmt.Errorf("failed to remove setup flag: %w", err)
	}

	h.sendProgress(progressChan, 6, 6, "Setup complete",
		fmt.Sprintf("✅ Initial setup completed successfully for %s!", config.Name),
		"Application is ready for use")

	return nil
}

// extractImages extracts all image specifications from docker-compose content
func (h *InitialSetupHandler) extractImages(composeContent []byte) ([]string, error) {
	var compose map[string]interface{}
	if err := yaml.Unmarshal(composeContent, &compose); err != nil {
		return nil, err
	}

	images := []string{}
	services, ok := compose["services"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no services found in docker-compose")
	}

	for _, service := range services {
		serviceMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}
		if image, ok := serviceMap["image"].(string); ok {
			images = append(images, image)
		}
	}

	return images, nil
}

// updateImagesToLatestVersions updates image tags to specific version numbers
func (h *InitialSetupHandler) updateImagesToLatestVersions(content string, images []string) (string, map[string]string, error) {
	versionMap := make(map[string]string)
	updatedContent := content

	for _, image := range images {
		// Get the latest version tag for this image
		latestVersion, err := h.getLatestVersionTag(image)
		if err != nil {
			log.Printf("Warning: Could not get latest version for %s: %v", image, err)
			continue
		}

		if latestVersion != "" && latestVersion != image {
			// Replace in content
			updatedContent = strings.ReplaceAll(updatedContent,
				fmt.Sprintf("image: %s", image),
				fmt.Sprintf("image: %s", latestVersion))
			versionMap[image] = latestVersion
		}
	}

	return updatedContent, versionMap, nil
}

// getLatestVersionTag attempts to find the latest version tag for an image
func (h *InitialSetupHandler) getLatestVersionTag(image string) (string, error) {
	// For common images, we can use specific strategies
	if strings.Contains(image, "open-webui") {
		// For OpenWebUI, try to get the latest release version
		return h.getOpenWebUILatestVersion(image)
	}

	// For other images, keep the existing tag for now
	// In the future, this could query Docker Hub API or other registries
	return image, nil
}

// getOpenWebUILatestVersion gets the latest version for OpenWebUI images
func (h *InitialSetupHandler) getOpenWebUILatestVersion(image string) (string, error) {
	// Parse the current image
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return image, nil
	}

	baseImage := parts[0]
	// currentTag := parts[1] // Keep for future use

	// Try to pull the image to get version info
	cmd := exec.Command("docker", "pull", image)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return image, fmt.Errorf("failed to pull image: %w", err)
	}

	// Try to extract version from the image labels
	inspectCmd := exec.Command("docker", "inspect", image,
		"--format={{index .Config.Labels \"org.opencontainers.image.version\"}}")
	versionOutput, err := inspectCmd.Output()
	if err == nil && len(versionOutput) > 0 {
		version := strings.TrimSpace(string(versionOutput))
		if version != "" && version != "<no value>" {
			// Return the image with the specific version tag
			return fmt.Sprintf("%s:%s", baseImage, version), nil
		}
	}

	// If we can't find a version, check if there's a latest stable version
	// For now, keep the original tag
	return image, nil
}

// pullImages pulls all Docker images for the app
func (h *InitialSetupHandler) pullImages(ctx context.Context, appPath string, progressChan chan<- SetupProgress) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "pull")
	cmd.Dir = appPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose pull failed: %w\nOutput: %s", err, output)
	}

	h.sendProgress(progressChan, 4, 6, "Pulling Docker images",
		"Images pulled successfully", string(output))

	return nil
}

// startContainers starts the Docker containers
func (h *InitialSetupHandler) startContainers(ctx context.Context, appPath string, progressChan chan<- SetupProgress) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
	cmd.Dir = appPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %w\nOutput: %s", err, output)
	}

	h.sendProgress(progressChan, 5, 6, "Starting containers",
		"Containers started successfully", string(output))

	return nil
}

// removeSetupFlag removes the initial_setup_required flag from app.yml
func (h *InitialSetupHandler) removeSetupFlag(appPath string) error {
	appYmlPath := filepath.Join(appPath, "app.yml")

	// Read current app.yml
	data, err := os.ReadFile(appYmlPath)
	if err != nil {
		return fmt.Errorf("failed to read app.yml: %w", err)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse app.yml: %w", err)
	}

	// Remove the flag
	delete(config, "initial_setup_required")

	// Write back
	updatedData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal app.yml: %w", err)
	}

	if err := os.WriteFile(appYmlPath, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write app.yml: %w", err)
	}

	return nil
}

// Helper methods for progress reporting
func (h *InitialSetupHandler) sendProgress(progressChan chan<- SetupProgress, step, total int, name, message, details string) {
	if progressChan != nil {
		progressChan <- SetupProgress{
			Step:       step,
			TotalSteps: total,
			StepName:   name,
			Message:    message,
			Details:    details,
			IsError:    false,
		}
	}
}

func (h *InitialSetupHandler) sendError(progressChan chan<- SetupProgress, step, total int, message, details string) {
	if progressChan != nil {
		progressChan <- SetupProgress{
			Step:       step,
			TotalSteps: total,
			StepName:   "Error",
			Message:    message,
			Details:    details,
			IsError:    true,
		}
	}
}
