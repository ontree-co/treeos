package compose

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/docker/docker/client"
)

// Service wraps the Docker Compose SDK functionality
type Service struct {
	service      api.Service
	dockerClient *client.Client
}

// NewService creates a new instance of the compose service
func NewService() (*Service, error) {
	// Create a Docker CLI instance
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker cli: %w", err)
	}

	// Initialize the Docker CLI with default options
	opts := flags.NewClientOptions()
	if err := dockerCli.Initialize(opts); err != nil {
		return nil, fmt.Errorf("failed to initialize docker cli: %w", err)
	}

	// Create a standard Docker client for direct container operations
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Create the compose service
	composeService := compose.NewComposeService(dockerCli)

	return &Service{
		service:      composeService,
		dockerClient: dockerClient,
	}, nil
}

// Close closes the Docker client connections
func (s *Service) Close() error {
	if s.dockerClient != nil {
		return s.dockerClient.Close()
	}
	return nil
}

// Options represents options for compose operations
type Options struct {
	ProjectName string
	WorkingDir  string
	EnvFile     string
}

// Up starts a compose project (equivalent to docker-compose up)
func (s *Service) Up(ctx context.Context, opts Options) error {
	project, err := s.loadProject(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// First, try to start with no recreation (handles most cases efficiently)
	upOptions := api.UpOptions{
		Create: api.CreateOptions{
			RemoveOrphans: true,
			Recreate:      api.RecreateNever, // Never recreate, just start existing containers
		},
		Start: api.StartOptions{
			Wait:   false,
			Attach: nil,
		},
	}

	err = s.service.Up(ctx, project, upOptions)

	// Handle specific error cases
	if err != nil && strings.Contains(err.Error(), "no container found") {
		// This can happen when containers are in a "created" but not "started" state
		// Try to start the containers without creating them
		startOpts := api.StartOptions{
			Wait:   false,
			Attach: nil,
		}
		startErr := s.service.Start(ctx, project.Name, startOpts)
		if startErr == nil {
			// Successfully started existing containers
			return nil
		}
		// If start didn't work, return the original error
	}

	return err
}

// Down stops and removes a compose project (equivalent to docker-compose down)
func (s *Service) Down(ctx context.Context, opts Options, removeVolumes bool) error {
	project, err := s.loadProject(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Stop and remove the services
	downOptions := api.DownOptions{
		RemoveOrphans: true,
		Volumes:       removeVolumes,
	}

	return s.service.Down(ctx, project.Name, downOptions)
}

// PS lists containers for a compose project (equivalent to docker-compose ps)
func (s *Service) PS(ctx context.Context, opts Options) ([]api.ContainerSummary, error) {
	project, err := s.loadProject(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	// List containers for the project
	psOptions := api.PsOptions{
		All: true, // Include stopped containers
	}

	return s.service.Ps(ctx, project.Name, psOptions)
}

// LogWriter provides a way to write logs to custom writers
type LogWriter struct {
	Out io.Writer
	Err io.Writer
}

// LogConsumerWriter implements the api.LogConsumer interface
type LogConsumerWriter struct {
	writer LogWriter
}

// Log handles normal log messages
func (l *LogConsumerWriter) Log(containerName, message string) {
	// Extract service name from container name (format: ontree-{appName}-{serviceName}-{index})
	serviceName := containerName
	parts := strings.Split(containerName, "-")
	if len(parts) >= 3 {
		// Take the service name part(s), excluding "ontree", app name, and index
		serviceName = strings.Join(parts[2:len(parts)-1], "-")
	}

	// Format: [service-name] message
	fmt.Fprintf(l.writer.Out, "[%s] %s", serviceName, message)
}

// Status handles status messages
func (l *LogConsumerWriter) Status(container, message string) {
	// Status messages (container started/stopped)
	fmt.Fprintf(l.writer.Out, "Status: %s\n", message)
}

// Err handles error messages
func (l *LogConsumerWriter) Err(container, message string) {
	// Error messages
	fmt.Fprintf(l.writer.Err, "Error: %s\n", message)
}

// Register is required by the LogConsumer interface
func (l *LogConsumerWriter) Register(container string) {
	// No-op - we don't need to register containers
}

// Logs streams logs from a compose project (equivalent to docker-compose logs)
func (s *Service) Logs(ctx context.Context, opts Options, services []string, follow bool, writer LogWriter) error {
	project, err := s.loadProject(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Set up log options
	logOptions := api.LogOptions{
		Services:   services,
		Follow:     follow,
		Timestamps: true,
		Tail:       "all",
	}

	consumer := &LogConsumerWriter{
		writer: writer,
	}

	return s.service.Logs(ctx, project.Name, consumer, logOptions)
}

// loadProject loads a compose project from the specified directory
func (s *Service) loadProject(ctx context.Context, opts Options) (*types.Project, error) {
	// Determine compose file path
	composeFile := filepath.Join(opts.WorkingDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		// Try docker-compose.yaml as fallback
		composeFile = filepath.Join(opts.WorkingDir, "docker-compose.yaml")
		if _, err := os.Stat(composeFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("no docker-compose.yml or docker-compose.yaml found in %s", opts.WorkingDir)
		}
	}

	// Set up config details with proper environment
	configDetails := types.ConfigDetails{
		WorkingDir: opts.WorkingDir,
		ConfigFiles: []types.ConfigFile{
			{
				Filename: composeFile,
			},
		},
		Environment: map[string]string{
			"COMPOSE_PROJECT_NAME": opts.ProjectName,
		},
	}

	// If env file is specified, load it
	if opts.EnvFile != "" {
		envFile := filepath.Join(opts.WorkingDir, opts.EnvFile)
		if _, err := os.Stat(envFile); err == nil {
			configDetails.ConfigFiles = append(configDetails.ConfigFiles, types.ConfigFile{
				Filename: envFile,
			})
		}
	}

	// Load the project with explicit project name
	project, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		options.SetProjectName(opts.ProjectName, true)
		// Don't skip interpolation - we want environment variables to be used
		options.SkipInterpolation = false
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose file: %w", err)
	}

	// Ensure project name is set
	project.Name = opts.ProjectName
	
	// Apply project labels to all services
	// This ensures containers get the correct compose labels
	for serviceName, service := range project.Services {
		if service.CustomLabels == nil {
			service.CustomLabels = make(map[string]string)
		}
		// Add essential Docker Compose labels
		service.CustomLabels["com.docker.compose.project"] = project.Name
		service.CustomLabels["com.docker.compose.service"] = serviceName
		
		// Update the service in the project
		project.Services[serviceName] = service
	}

	return project, nil
}
