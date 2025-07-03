package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/image"
)

// Service wraps the Docker client with app directory configuration
type Service struct {
	client  *Client
	appsDir string
}

// NewService creates a new Docker service
func NewService(appsDir string) (*Service, error) {
	client, err := NewClient()
	if err != nil {
		return nil, err
	}
	
	return &Service{
		client:  client,
		appsDir: appsDir,
	}, nil
}

// Close closes the Docker service
func (s *Service) Close() error {
	return s.client.Close()
}

// ScanApps delegates to the client with the configured apps directory
func (s *Service) ScanApps() ([]*App, error) {
	return s.client.ScanApps(s.appsDir)
}

// GetAppDetails delegates to the client with the configured apps directory
func (s *Service) GetAppDetails(appName string) (*App, error) {
	return s.client.GetAppDetails(s.appsDir, appName)
}

// StartApp delegates to the client with the configured apps directory
func (s *Service) StartApp(appName string) error {
	return s.client.StartApp(s.appsDir, appName)
}

// StopApp delegates to the client
func (s *Service) StopApp(appName string) error {
	return s.client.StopApp(appName)
}

// RecreateApp delegates to the client with the configured apps directory
func (s *Service) RecreateApp(appName string) error {
	return s.client.RecreateApp(s.appsDir, appName)
}

// DeleteApp delegates to the client
func (s *Service) DeleteApp(appName string) error {
	return s.client.DeleteAppContainer(appName)
}

// ProgressCallback is called with progress updates during image operations
type ProgressCallback func(progress int, message string)

// PullImagesWithProgress pulls Docker images for an app with progress reporting
func (s *Service) PullImagesWithProgress(appName string, progressCallback ProgressCallback) error {
	// Get app details to find the image
	app, err := s.GetAppDetails(appName)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}
	
	if app.Config == nil || app.Config.Container.Image == "" {
		return fmt.Errorf("no image configured for app: %s", appName)
	}
	
	ctx := context.Background()
	imageName := app.Config.Container.Image
	
	// Start pulling the image
	reader, err := s.client.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	
	// Parse the JSON stream for progress updates
	decoder := json.NewDecoder(reader)
	var totalProgress int
	layerProgress := make(map[string]int)
	
	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode progress: %w", err)
		}
		
		// Extract progress information
		if status, ok := event["status"].(string); ok {
			id, _ := event["id"].(string)
			
			// Handle different status messages
			switch {
			case strings.HasPrefix(status, "Pulling from"):
				progressCallback(0, fmt.Sprintf("Pulling image %s", imageName))
			
			case status == "Downloading":
				if progressDetail, ok := event["progressDetail"].(map[string]interface{}); ok {
					if current, ok := progressDetail["current"].(float64); ok {
						if total, ok := progressDetail["total"].(float64); ok && total > 0 {
							layerProgress[id] = int((current / total) * 100)
						}
					}
				}
				
				// Calculate overall progress
				if len(layerProgress) > 0 {
					sum := 0
					for _, progress := range layerProgress {
						sum += progress
					}
					totalProgress = sum / len(layerProgress)
					progressCallback(totalProgress, fmt.Sprintf("Downloading layers... %d%%", totalProgress))
				}
			
			case status == "Download complete":
				layerProgress[id] = 100
			
			case status == "Extracting":
				progressCallback(90, "Extracting layers...")
			
			case strings.Contains(status, "Pull complete"):
				progressCallback(95, "Finalizing...")
			
			case strings.Contains(status, "Downloaded newer image"):
				progressCallback(100, "Image pull completed")
			
			case strings.Contains(status, "Image is up to date"):
				progressCallback(100, "Image is up to date")
			}
		}
	}
	
	// Ensure we report 100% completion
	progressCallback(100, "Image ready")
	
	return nil
}