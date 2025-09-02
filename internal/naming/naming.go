package naming

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SystemPrefix is the hardcoded prefix for all containers managed by this system
	SystemPrefix = "ontree"

	// BasePath is the base directory where all apps are stored
	BasePath = "/opt/ontree/apps"

	// Separator is the character used to join naming components
	Separator = "-"
)

// GetAppIdentifier extracts and normalizes the app identifier from a directory path
// For example: /opt/ontree/apps/Code-Server -> code-server
func GetAppIdentifier(appPath string) string {
	// Get the base name of the path
	appName := filepath.Base(appPath)

	// Convert to lowercase
	return strings.ToLower(appName)
}

// GetComposeProjectName generates the Docker Compose project name for an app
// Pattern: ontree-<app_identifier>
func GetComposeProjectName(appIdentifier string) string {
	return fmt.Sprintf("%s%s%s", SystemPrefix, Separator, appIdentifier)
}

// GetContainerName generates the expected container name for a service
// Pattern: ontree-<app_identifier>-<service_name>-<instance_number>
func GetContainerName(appIdentifier, serviceName string, instanceNumber int) string {
	projectName := GetComposeProjectName(appIdentifier)
	return fmt.Sprintf("%s%s%s%s%d", projectName, Separator, serviceName, Separator, instanceNumber)
}

// GetContainerPattern returns a pattern to match all containers for an app
// Pattern: ontree-<app_identifier>-*
func GetContainerPattern(appIdentifier string) string {
	projectName := GetComposeProjectName(appIdentifier)
	return fmt.Sprintf("%s%s*", projectName, Separator)
}

// GenerateEnvFile creates a .env file with the required Docker Compose configuration
func GenerateEnvFile(appPath string) error {
	// Get app identifier
	appIdentifier := GetAppIdentifier(appPath)

	// Generate project name
	projectName := GetComposeProjectName(appIdentifier)

	// Create .env content
	envContent := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s\nCOMPOSE_SEPARATOR=%s\n", projectName, Separator)

	// Write to .env file
	envPath := filepath.Join(appPath, ".env")
	return os.WriteFile(envPath, []byte(envContent), 0644)
}

// EnsureEnvFile checks if .env exists and creates it if not
func EnsureEnvFile(appPath string) error {
	envPath := filepath.Join(appPath, ".env")

	// Check if .env already exists
	if _, err := os.Stat(envPath); err == nil {
		// File exists, check if it has the correct content
		content, err := os.ReadFile(envPath)
		if err != nil {
			return fmt.Errorf("failed to read existing .env: %w", err)
		}

		// Check if it already has COMPOSE_PROJECT_NAME
		if strings.Contains(string(content), "COMPOSE_PROJECT_NAME=") {
			// Already configured, nothing to do
			return nil
		}
	}

	// Generate new .env file
	return GenerateEnvFile(appPath)
}

// GetAppID generates the internal app ID used in databases
// Pattern: <app_identifier> (no prefix for internal use)
func GetAppID(appIdentifier string) string {
	// For internal consistency, we'll use just the app identifier
	// This matches the directory name pattern
	return appIdentifier
}

// ParseContainerName extracts components from a container name
// Returns appIdentifier, serviceName, instanceNumber, and error
func ParseContainerName(containerName string) (string, string, int, error) {
	// Remove leading slash if present
	containerName = strings.TrimPrefix(containerName, "/")

	// Check if it starts with our prefix
	if !strings.HasPrefix(containerName, SystemPrefix+Separator) {
		return "", "", 0, fmt.Errorf("container name does not start with %s%s", SystemPrefix, Separator)
	}

	// Remove prefix
	withoutPrefix := strings.TrimPrefix(containerName, SystemPrefix+Separator)

	// Split remaining parts
	parts := strings.Split(withoutPrefix, Separator)
	if len(parts) < 3 {
		return "", "", 0, fmt.Errorf("invalid container name format: %s", containerName)
	}

	// Last part should be the instance number
	var instanceNumber int
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &instanceNumber); err != nil {
		return "", "", 0, fmt.Errorf("invalid instance number: %s", parts[len(parts)-1])
	}

	// Service name is the second to last part
	serviceName := parts[len(parts)-2]

	// App identifier is everything else joined back together
	appIdentifier := strings.Join(parts[:len(parts)-2], Separator)

	return appIdentifier, serviceName, instanceNumber, nil
}

// IsOurContainer checks if a container name belongs to our system
func IsOurContainer(containerName string) bool {
	containerName = strings.TrimPrefix(containerName, "/")
	return strings.HasPrefix(containerName, SystemPrefix+Separator)
}
