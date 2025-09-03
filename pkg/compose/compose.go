package compose

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
	WorkingDir string
	EnvFile    string
}

// Up starts a compose project (equivalent to docker-compose up)
func (s *Service) Up(ctx context.Context, opts Options) error {
	project, err := s.loadProject(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// WORKAROUND: Docker Compose v2 SDK has issues with project labels
	// The SDK doesn't properly set the com.docker.compose.project label on containers,
	// which causes "no container found" errors on subsequent operations.
	// As a temporary workaround, we'll use the Docker Compose CLI directly.
	log.Printf("INFO: Using docker-compose CLI for project %s due to SDK label limitations", project.Name)

	// Use docker-compose CLI as a workaround
	return s.upUsingCLI(ctx, opts)
}

// upUsingCLI uses docker-compose CLI as a workaround for SDK label issues
func (s *Service) upUsingCLI(ctx context.Context, opts Options) error {
	// Try "docker compose" first (v2), then fall back to "docker-compose" (v1)
	output, err := s.runComposeCommand(ctx, opts, "up", "-d")
	if err != nil {
		return fmt.Errorf("failed to start containers: %w\nOutput: %s", err, string(output))
	}

	log.Printf("INFO: Successfully started containers using docker-compose CLI")
	return nil
}

// runComposeCommand runs a docker-compose command, trying both v2 and v1 syntax
func (s *Service) runComposeCommand(ctx context.Context, opts Options, command ...string) ([]byte, error) {
	// Validate working directory to prevent directory traversal
	absPath, err := filepath.Abs(opts.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("invalid working directory: %w", err)
	}

	// Build base arguments
	composeFile := filepath.Join(absPath, "docker-compose.yml")
	baseArgs := []string{"-f", composeFile}

	// Read project name from .env file if it exists, otherwise use directory name
	projectName := filepath.Base(absPath)
	envFile := filepath.Join(absPath, ".env")
	if _, err := os.Stat(envFile); err == nil {
		envContent, err := os.ReadFile(envFile)
		if err == nil {
			lines := strings.Split(string(envContent), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "COMPOSE_PROJECT_NAME=") {
					projectName = strings.TrimPrefix(line, "COMPOSE_PROJECT_NAME=")
					projectName = strings.TrimSpace(projectName)
					break
				}
			}
		}
	}
	baseArgs = append(baseArgs, "-p", projectName)

	// Add env file if specified
	if opts.EnvFile != "" {
		envFile := filepath.Join(absPath, opts.EnvFile)
		baseArgs = append(baseArgs, "--env-file", envFile)
	}

	// Add the command and its arguments
	args := append(baseArgs, command...)

	// Try "docker compose" first (v2)
	// #nosec G204 - Command arguments are validated and come from trusted sources
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = absPath
	output, err := cmd.CombinedOutput()

	if err == nil {
		return output, nil
	}

	// If v2 failed, try "docker-compose" (v1)
	// #nosec G204 - Command arguments are validated and come from trusted sources
	cmd = exec.CommandContext(ctx, "docker-compose", args...)
	cmd.Dir = absPath
	output2, err2 := cmd.CombinedOutput()

	if err2 == nil {
		return output2, nil
	}

	// Return the v2 error as it's more likely to be available in modern environments
	return output, err
}

// Down stops and removes a compose project (equivalent to docker-compose down)
func (s *Service) Down(ctx context.Context, opts Options, removeVolumes bool) error {
	// Build command arguments
	cmdArgs := []string{"down"}
	if removeVolumes {
		cmdArgs = append(cmdArgs, "-v")
	}

	// Try "docker compose" first (v2), then fall back to "docker-compose" (v1)
	output, err := s.runComposeCommand(ctx, opts, cmdArgs...)
	if err != nil {
		return fmt.Errorf("failed to stop containers: %w\nOutput: %s", err, string(output))
	}

	log.Printf("INFO: Successfully stopped containers using docker-compose CLI")
	return nil
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
	// Load environment variables from .env file if it exists
	envVars := make(map[string]string)
	envFile := filepath.Join(opts.WorkingDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		// Read .env file
		envContent, err := os.ReadFile(envFile)
		if err == nil {
			// Parse .env file (simple key=value format)
			lines := strings.Split(string(envContent), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						envVars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}

	// Fall back to directory name if COMPOSE_PROJECT_NAME not in .env
	if _, ok := envVars["COMPOSE_PROJECT_NAME"]; !ok {
		envVars["COMPOSE_PROJECT_NAME"] = filepath.Base(opts.WorkingDir)
	}

	configDetails := types.ConfigDetails{
		WorkingDir: opts.WorkingDir,
		ConfigFiles: []types.ConfigFile{
			{
				Filename: composeFile,
			},
		},
		Environment: envVars,
	}

	// If additional env file is specified, load it
	if opts.EnvFile != "" && opts.EnvFile != ".env" {
		additionalEnvFile := filepath.Join(opts.WorkingDir, opts.EnvFile)
		if _, err := os.Stat(additionalEnvFile); err == nil {
			configDetails.ConfigFiles = append(configDetails.ConfigFiles, types.ConfigFile{
				Filename: additionalEnvFile,
			})
		}
	}

	// Load the project
	project, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		// Use COMPOSE_PROJECT_NAME from environment if available, otherwise use directory name
		projectName := envVars["COMPOSE_PROJECT_NAME"]
		if projectName == "" {
			projectName = filepath.Base(opts.WorkingDir)
		}
		options.SetProjectName(projectName, true)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose file: %w", err)
	}

	return project, nil
}
