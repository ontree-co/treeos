package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/image"
	"go.opentelemetry.io/otel/attribute"
	"treeos/internal/telemetry"
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
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "docker.scan_apps")
	defer span.End()

	span.SetAttributes(
		attribute.String("apps.dir", s.appsDir),
	)

	apps, err := s.client.ScanApps(s.appsDir)
	if err != nil {
		span.RecordError(err)
	} else {
		span.SetAttributes(
			attribute.Int("apps.count", len(apps)),
		)
	}
	return apps, err
}

// GetAppDetails delegates to the client with the configured apps directory
func (s *Service) GetAppDetails(appName string) (*App, error) {
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "docker.get_app_details")
	defer span.End()

	span.SetAttributes(
		attribute.String("app.name", appName),
		attribute.String("apps.dir", s.appsDir),
	)

	app, err := s.client.GetAppDetails(s.appsDir, appName)
	if err != nil {
		span.RecordError(err)
	}
	return app, err
}

// ProgressCallback is called with progress updates during image operations
type ProgressCallback func(progress int, message string)

// PullImagesWithProgress pulls Docker images for an app with progress reporting
func (s *Service) PullImagesWithProgress(appName string, progressCallback ProgressCallback) error {
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "docker.pull_images")
	defer span.End()

	span.SetAttributes(
		attribute.String("app.name", appName),
	)

	// Get app details to find the images
	app, err := s.GetAppDetails(appName)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get app details: %w", err)
	}

	if len(app.Services) == 0 {
		return fmt.Errorf("no services configured for app: %s", appName)
	}

	// Pull images for all services
	for serviceName, service := range app.Services {
		if service.Image == "" {
			continue
		}
		imageName := service.Image
		span.SetAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("image.name", imageName),
		)

		progressCallback(0, fmt.Sprintf("Pulling image for service %s: %s", serviceName, imageName))

		// Start pulling the image
		reader, err := s.client.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to pull image for service %s: %w", serviceName, err)
		}
		defer func() {
			if err := reader.Close(); err != nil {
				// Log error but don't fail the operation
				fmt.Printf("Failed to close reader: %v\n", err)
			}
		}()

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
				id, ok := event["id"].(string)
				if !ok {
					id = ""
				}

				// Handle different status messages
				switch {
				case strings.HasPrefix(status, "Pulling from"):
					progressCallback(0, fmt.Sprintf("[%s] Pulling image %s", serviceName, imageName))

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
						progressCallback(totalProgress, fmt.Sprintf("[%s] Downloading layers... %d%%", serviceName, totalProgress))
					}

				case status == "Download complete":
					layerProgress[id] = 100

				case status == "Extracting":
					progressCallback(90, fmt.Sprintf("[%s] Extracting layers...", serviceName))

				case strings.Contains(status, "Pull complete"):
					progressCallback(95, fmt.Sprintf("[%s] Finalizing...", serviceName))

				case strings.Contains(status, "Downloaded newer image"):
					progressCallback(100, fmt.Sprintf("[%s] Image pull completed", serviceName))

				case strings.Contains(status, "Image is up to date"):
					progressCallback(100, fmt.Sprintf("[%s] Image is up to date", serviceName))
				}
			}
		}

		// Ensure we report 100% completion for this service
		progressCallback(100, fmt.Sprintf("[%s] Image ready", serviceName))
	}

	return nil
}