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
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
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
	WorkingDir string
	EnvFile    string
}

// Up starts a compose project (equivalent to docker-compose up)
func (s *Service) Up(ctx context.Context, opts Options) error {
	project, err := s.loadProject(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// First check if containers already exist for this project
	existingContainers, err := s.service.Ps(ctx, project.Name, api.PsOptions{All: true})
	if err == nil && len(existingContainers) > 0 {
		// Containers exist, just start them
		for _, cont := range existingContainers {
			if cont.State != "running" {
				// Use docker client directly to start the container
				err := s.dockerClient.ContainerStart(ctx, cont.ID, container.StartOptions{})
				if err != nil {
					return fmt.Errorf("failed to start container %s: %w", cont.Name, err)
				}
			}
		}
		return nil
	}

	// No existing containers, create and start them
	err = s.service.Create(ctx, project, api.CreateOptions{
		RemoveOrphans: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create containers: %w", err)
	}

	// Now start the containers
	err = s.service.Start(ctx, project.Name, api.StartOptions{})
	if err != nil {
		// If we get "no container found" error, try starting containers directly
		if strings.Contains(err.Error(), "no container found") {
			// List all containers and start them directly
			filters := filters.NewArgs()
			filters.Add("label", fmt.Sprintf("com.docker.compose.project=%s", project.Name))
			containers, err := s.dockerClient.ContainerList(ctx, container.ListOptions{
				All:     true,
				Filters: filters,
			})
			if err == nil {
				for _, c := range containers {
					if c.State != "running" {
						err := s.dockerClient.ContainerStart(ctx, c.ID, container.StartOptions{})
						if err != nil {
							return fmt.Errorf("failed to start container %s: %w", c.Names[0], err)
						}
					}
				}
				return nil
			}
		}
		return fmt.Errorf("failed to start containers: %w", err)
	}

	return nil
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

	// Set up config details
	configDetails := types.ConfigDetails{
		WorkingDir: opts.WorkingDir,
		ConfigFiles: []types.ConfigFile{
			{
				Filename: composeFile,
			},
		},
		Environment: map[string]string{
			"COMPOSE_PROJECT_NAME": filepath.Base(opts.WorkingDir),
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

	// Load the project
	project, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		// Set the project name from the directory name
		options.SetProjectName(filepath.Base(opts.WorkingDir), true)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose file: %w", err)
	}

	return project, nil
}
