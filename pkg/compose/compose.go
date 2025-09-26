package compose

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Service wraps access to podman compose operations (Podman 4+ required).
type Service struct {
	podmanBinary string
}

// NewService creates a new compose service instance (requires Podman 4+ with built-in compose).
func NewService() (*Service, error) {
	podmanBin := os.Getenv("PODMAN_BINARY")
	if podmanBin == "" {
		podmanBin = "podman"
	}

	// Check that Podman is installed
	if err := commandAvailable(podmanBin, "--version"); err != nil {
		return nil, fmt.Errorf("podman not found: %w", err)
	}

	// Require Podman 4+ with built-in compose support
	if err := commandAvailable(podmanBin, "compose", "--help"); err != nil {
		return nil, fmt.Errorf("podman 4+ required (built-in compose not found). Please upgrade to Podman 4.0 or later")
	}

	return &Service{
		podmanBinary: podmanBin,
	}, nil
}

// Close tears down resources. Present for API compatibility.
func (s *Service) Close() error { return nil }

// Options represents options for compose operations.
type Options struct {
	WorkingDir string
	EnvFile    string
}

// ContainerSummary captures container state returned by podman.
type ContainerSummary struct {
	ID      string
	Name    string
	Service string
	State   string
	Status  string
	Image   string
	Health  string
	Ports   []PortMapping
}

// PortMapping represents a host/container port binding reported by podman ps.
type PortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string
}

// ProgressCallback is called for each line of output during container operations
type ProgressCallback func(line string)

// Up starts a compose project (equivalent to `podman compose up -d`).
func (s *Service) Up(ctx context.Context, opts Options) error {
	return s.UpWithProgress(ctx, opts, nil)
}

// UpWithProgress starts a compose project with progress monitoring.
func (s *Service) UpWithProgress(ctx context.Context, opts Options, progressCallback ProgressCallback) error {
	cmd, err := s.newComposeCmd(ctx, opts, "up", "-d")
	if err != nil {
		return err
	}

	if progressCallback == nil {
		// Fallback to simple execution if no progress callback
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to start containers: %w (output: %s)", err, strings.TrimSpace(string(output)))
		}
		return nil
	}

	// Set up pipes to capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Process output streams concurrently
	outputChan := make(chan string, 100)
	errorChan := make(chan error, 2)

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			outputChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			errorChan <- fmt.Errorf("stdout scan error: %w", err)
		} else {
			errorChan <- nil
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			outputChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			errorChan <- fmt.Errorf("stderr scan error: %w", err)
		} else {
			errorChan <- nil
		}
	}()

	// Process output lines and call progress callback
	go func() {
		for line := range outputChan {
			progressCallback(line)
		}
	}()

	// Wait for both readers to finish
	var readErrors []error
	for i := 0; i < 2; i++ {
		if err := <-errorChan; err != nil {
			readErrors = append(readErrors, err)
		}
	}

	// Close output channel
	close(outputChan)

	// Wait for command to complete
	cmdErr := cmd.Wait()

	// Check for read errors first
	if len(readErrors) > 0 {
		return fmt.Errorf("failed to read command output: %v", readErrors)
	}

	// Check command execution error
	if cmdErr != nil {
		return fmt.Errorf("failed to start containers: %w", cmdErr)
	}

	return nil
}

// Down stops a compose project (equivalent to `podman compose down`).
func (s *Service) Down(ctx context.Context, opts Options, removeVolumes bool) error {
	args := []string{"down"}
	if removeVolumes {
		args = append(args, "--volumes")
	}

	cmd, err := s.newComposeCmd(ctx, opts, args...)
	if err != nil {
		return err
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop containers: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// PS lists containers belonging to the compose project.
func (s *Service) PS(ctx context.Context, opts Options) ([]ContainerSummary, error) {
	absPath, projectName, err := resolveProject(opts)
	if err != nil {
		return nil, err
	}

	// Collect containers using podman ps with the compose label.
	summaries, err := s.listContainersForProject(ctx, projectName)
	if err != nil {
		return nil, err
	}

	// Provide deterministic order based on container name to keep UI stable.
	sortContainerSummaries(summaries)

	// Ensure WorkingDir exists to match previous behaviour.
	_ = absPath

	return summaries, nil
}

// LogWriter captures stdout/stderr destinations for compose logs.
type LogWriter struct {
	Out io.Writer
	Err io.Writer
}

// Logs streams logs from the compose project using the podman compose CLI.
func (s *Service) Logs(ctx context.Context, opts Options, services []string, follow bool, writer LogWriter) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	if len(services) > 0 {
		args = append(args, services...)
	}

	cmd, err := s.newComposeCmd(ctx, opts, args...)
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to attach stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to attach stderr: %w", err)
	}

	if writer.Out == nil {
		writer.Out = io.Discard
	}
	if writer.Err == nil {
		writer.Err = io.Discard
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start logs command: %w", err)
	}

	errCh := make(chan error, 2)
	go func() {
		_, copyErr := io.Copy(writer.Out, stdout)
		errCh <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(writer.Err, stderr)
		errCh <- copyErr
	}()

	waitErr := cmd.Wait()
	stdOutErr := <-errCh
	stdErrErr := <-errCh

	if stdOutErr != nil && !errors.Is(stdOutErr, context.Canceled) {
		return stdOutErr
	}
	if stdErrErr != nil && !errors.Is(stdErrErr, context.Canceled) {
		return stdErrErr
	}
	if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		return fmt.Errorf("logs command failed: %w", waitErr)
	}
	return nil
}

// --- helper functions ---

type podmanContainer struct {
	ID     string            `json:"Id"`
	Name   string            `json:"Name"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Image  string            `json:"Image"`
	Labels map[string]string `json:"Labels"`
	Ports  []struct {
		HostIP        string      `json:"host_ip"`
		HostPort      interface{} `json:"host_port"`
		ContainerPort interface{} `json:"container_port"`
		Protocol      string      `json:"protocol"`
	} `json:"Ports"`
	Health string `json:"Health"`
}

func commandAvailable(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	if len(args) == 0 {
		// Just check that the binary exists in PATH.
		if _, err := exec.LookPath(bin); err != nil {
			return err
		}
		return nil
	}
	// We only care about command availability, suppressing stdout/err.
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (s *Service) newComposeCmd(ctx context.Context, opts Options, extra ...string) (*exec.Cmd, error) {
	absPath, _, err := resolveProject(opts)
	if err != nil {
		return nil, err
	}

	composeFile, err := locateComposeFile(absPath)
	if err != nil {
		return nil, err
	}

	// Always check for .env file in the project directory
	envFile := filepath.Join(absPath, ".env")
	if opts.EnvFile != "" {
		envFile = filepath.Join(absPath, opts.EnvFile)
	}

	args := []string{"compose", "-f", composeFile}

	// Always pass env file if it exists
	// The .env file should contain COMPOSE_PROJECT_NAME
	if _, err := os.Stat(envFile); err == nil {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, extra...)

	// #nosec G204 -- command arguments constructed from validated project metadata
	cmd := exec.CommandContext(ctx, s.podmanBinary, args...)
	cmd.Dir = absPath
	return cmd, nil
}

func (s *Service) listContainersForProject(ctx context.Context, project string) ([]ContainerSummary, error) {
	// Try Podman labels first
	filters := []string{
		fmt.Sprintf("label=io.podman.compose.project=%s", project),
	}

	summaries, err := s.queryContainers(ctx, filters)
	if err != nil {
		return nil, err
	}

	// If no containers found, try Docker labels (podman-compose sets both)
	if len(summaries) == 0 {
		filters = []string{
			fmt.Sprintf("label=com.docker.compose.project=%s", project),
		}
		summaries, err = s.queryContainers(ctx, filters)
		if err != nil {
			return nil, err
		}
	}

	return summaries, nil
}

func (s *Service) queryContainers(ctx context.Context, filters []string) ([]ContainerSummary, error) {
	args := []string{"ps", "--all"}
	for _, filter := range filters {
		args = append(args, "--filter", filter)
	}
	args = append(args, "--format", "json")

	// #nosec G204 -- arguments are generated internally for podman interaction
	cmd := exec.CommandContext(ctx, s.podmanBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("podman ps failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []ContainerSummary{}, nil
	}

	var containers []podmanContainer
	if err := json.Unmarshal([]byte(trimmed), &containers); err != nil {
		return nil, fmt.Errorf("failed to parse podman ps output: %w", err)
	}

	summaries := make([]ContainerSummary, 0, len(containers))
	for _, cont := range containers {
		// Check both labels as podman-compose sets both
		serviceName := cont.Labels["io.podman.compose.service"]
		if serviceName == "" {
			serviceName = cont.Labels["com.docker.compose.service"]
		}

		name := cont.Name
		if name == "" && len(cont.Names) > 0 {
			name = cont.Names[0]
		}
		name = strings.TrimPrefix(name, "/")

		ports := make([]PortMapping, 0, len(cont.Ports))
		for _, port := range cont.Ports {
			// Convert port numbers to strings
			hostPort := fmt.Sprintf("%v", port.HostPort)
			containerPort := fmt.Sprintf("%v", port.ContainerPort)
			ports = append(ports, PortMapping{
				HostIP:        port.HostIP,
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Protocol:      port.Protocol,
			})
		}

		summaries = append(summaries, ContainerSummary{
			ID:      cont.ID,
			Name:    name,
			Service: serviceName,
			State:   cont.State,
			Status:  cont.Status,
			Image:   cont.Image,
			Health:  cont.Health,
			Ports:   ports,
		})
	}

	return summaries, nil
}

func resolveProject(opts Options) (string, string, error) {
	if opts.WorkingDir == "" {
		return "", "", errors.New("working directory is required")
	}

	absPath, err := filepath.Abs(opts.WorkingDir)
	if err != nil {
		return "", "", fmt.Errorf("invalid working directory: %w", err)
	}

	projectName := projectNameFromEnv(absPath)
	if projectName == "" {
		projectName = sanitizeProjectName(filepath.Base(absPath))
	}

	return absPath, projectName, nil
}

func locateComposeFile(absPath string) (string, error) {
	candidates := []string{
		filepath.Join(absPath, "docker-compose.yml"),
		filepath.Join(absPath, "docker-compose.yaml"),
		filepath.Join(absPath, "compose.yml"),
		filepath.Join(absPath, "compose.yaml"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no compose file found in %s", absPath)
}

func projectNameFromEnv(dir string) string {
	envPath := filepath.Join(dir, ".env")
	file, err := os.Open(envPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "COMPOSE_PROJECT_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "COMPOSE_PROJECT_NAME="), "\" ")
		}
	}
	return ""
}

func sanitizeProjectName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastHyphen := false

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteRune('-')
				lastHyphen = true
			}
		}
	}

	project := strings.Trim(b.String(), "-")
	if project == "" {
		return name
	}
	return project
}

func sortContainerSummaries(containers []ContainerSummary) {
	sort.Slice(containers, func(i, j int) bool {
		if containers[i].Name == containers[j].Name {
			return containers[i].Service < containers[j].Service
		}
		return containers[i].Name < containers[j].Name
	})
}
