// Package docker provides Docker client functionality for managing containerized applications
package docker

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// BaseAppsDir is the base directory for all OnTree apps
	BaseAppsDir = "/opt/onTree/apps"
	// BaseMountDir is the base directory for all bind mounts
	BaseMountDir = "/opt/onTree/apps/mount"
)

// FileSystemManager handles file system operations for OnTree apps
type FileSystemManager struct{}

// NewFileSystemManager creates a new FileSystemManager instance
func NewFileSystemManager() *FileSystemManager {
	return &FileSystemManager{}
}

// ProvisionAppDirectories creates the necessary directory structure for a new app
// Creates:
// - /opt/onTree/apps/{appName}/
// - /opt/onTree/apps/mount/{appName}/
func (fs *FileSystemManager) ProvisionAppDirectories(appName string) error {
	// Validate app name
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	// Create app configuration directory
	appDir := filepath.Join(BaseAppsDir, appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	// Create app mount directory
	mountDir := filepath.Join(BaseMountDir, appName)
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	return nil
}

// ReadDockerComposeFile reads the docker-compose.yml file for an app
func (fs *FileSystemManager) ReadDockerComposeFile(appName string) ([]byte, error) {
	composePath := filepath.Join(BaseAppsDir, appName, "docker-compose.yml")
	
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read docker-compose.yml: %w", err)
	}
	
	return data, nil
}

// WriteDockerComposeFile writes the docker-compose.yml file for an app
func (fs *FileSystemManager) WriteDockerComposeFile(appName string, content []byte) error {
	composePath := filepath.Join(BaseAppsDir, appName, "docker-compose.yml")
	
	// Ensure the app directory exists
	appDir := filepath.Join(BaseAppsDir, appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}
	
	// Write the file with proper permissions
	if err := os.WriteFile(composePath, content, 0600); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}
	
	return nil
}

// ReadEnvFile reads the .env file for an app
func (fs *FileSystemManager) ReadEnvFile(appName string) ([]byte, error) {
	envPath := filepath.Join(BaseAppsDir, appName, ".env")
	
	// Check if file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// Return empty content if file doesn't exist (optional file)
		return []byte{}, nil
	}
	
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}
	
	return data, nil
}

// WriteEnvFile writes the .env file for an app
func (fs *FileSystemManager) WriteEnvFile(appName string, content []byte) error {
	envPath := filepath.Join(BaseAppsDir, appName, ".env")
	
	// Ensure the app directory exists
	appDir := filepath.Join(BaseAppsDir, appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}
	
	// If content is empty, remove the file if it exists
	if len(content) == 0 {
		if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove .env file: %w", err)
		}
		return nil
	}
	
	// Write the file with proper permissions
	if err := os.WriteFile(envPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}
	
	return nil
}

// GetAppPath returns the full path to an app's directory
func (fs *FileSystemManager) GetAppPath(appName string) string {
	return filepath.Join(BaseAppsDir, appName)
}

// GetAppMountPath returns the full path to an app's mount directory
func (fs *FileSystemManager) GetAppMountPath(appName string) string {
	return filepath.Join(BaseMountDir, appName)
}

// AppExists checks if an app directory exists
func (fs *FileSystemManager) AppExists(appName string) bool {
	appPath := fs.GetAppPath(appName)
	_, err := os.Stat(appPath)
	return err == nil
}

// DeleteAppDirectories removes all directories for an app
// This includes both the app configuration and mount directories
func (fs *FileSystemManager) DeleteAppDirectories(appName string) error {
	// Remove app configuration directory
	appDir := filepath.Join(BaseAppsDir, appName)
	if err := os.RemoveAll(appDir); err != nil {
		return fmt.Errorf("failed to remove app directory: %w", err)
	}
	
	// Remove app mount directory
	mountDir := filepath.Join(BaseMountDir, appName)
	if err := os.RemoveAll(mountDir); err != nil {
		return fmt.Errorf("failed to remove mount directory: %w", err)
	}
	
	return nil
}

// CreateServiceMountDirectory creates a mount directory for a specific service
func (fs *FileSystemManager) CreateServiceMountDirectory(appName, serviceName string) error {
	mountPath := filepath.Join(BaseMountDir, appName, serviceName)
	
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return fmt.Errorf("failed to create service mount directory: %w", err)
	}
	
	return nil
}