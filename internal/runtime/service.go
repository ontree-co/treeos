package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"treeos/internal/telemetry"
)

// Service wraps the runtime client with app directory configuration
type Service struct {
	client  *Client
	appsDir string
}

// NewService creates a new runtime service bound to the Podman CLI.
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

// Close closes the runtime service.
func (s *Service) Close() error {
	return s.client.Close()
}

// ScanApps delegates to the client with the configured apps directory
func (s *Service) ScanApps() ([]*App, error) {
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "runtime.scan_apps")
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
	_, span := telemetry.StartSpan(ctx, "runtime.get_app_details")
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

// PullImagesWithProgress pulls container images for an app with progress reporting.
func (s *Service) PullImagesWithProgress(appName string, progressCallback ProgressCallback) error {
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "runtime.pull_images")
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

		// #nosec G204 -- image names originate from validated compose files
		cmd := exec.CommandContext(ctx, s.client.binary(), "pull", imageName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to pull image for service %s: %w (output: %s)", serviceName, err, strings.TrimSpace(string(output)))
		}

		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			progressCallback(50, fmt.Sprintf("[%s] %s", serviceName, trimmed))
		}

		progressCallback(100, fmt.Sprintf("[%s] Image ready", serviceName))
	}

	return nil
}
