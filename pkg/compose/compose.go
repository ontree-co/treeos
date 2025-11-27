// Package compose provides a client for managing Docker Compose applications.
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

// Service wraps access to docker compose operations.
type Service struct {
	dockerBinary string
}

// NewService creates a new compose service instance.
func NewService() (*Service, error) {
	dockerBin := os.Getenv("DOCKER_BINARY")
	if dockerBin == "" {
		dockerBin = "docker"
	}

	// Check that Docker is installed
	if err := commandAvailable(dockerBin, "--version"); err != nil {
		return nil, fmt.Errorf("docker not found: %w", err)
	}

	// Check for Docker Compose support
	if err := commandAvailable(dockerBin, "compose", "--help"); err != nil {
		return nil, fmt.Errorf("docker compose not found. Please ensure Docker Compose is installed")
	}

	return &Service{
		dockerBinary: dockerBin,
	}, nil
}

// Close tears down resources. Present for API compatibility.
func (s *Service) Close() error { return nil }

// Options represents options for compose operations.
type Options struct {
	WorkingDir string
	EnvFile    string
}

// ContainerSummary captures container state returned by docker.
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

// PortMapping represents a host/container port binding reported by docker ps.
type PortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string
}

// ProgressCallback is called for each line of output during container operations
type ProgressCallback func(line string)

// Up starts a compose project (equivalent to `docker compose up -d`).
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

// Down stops a compose project (equivalent to `docker compose down`).
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

	// Collect containers using docker ps with the compose label.
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

// Logs streams logs from the compose project using the docker compose CLI.
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

type dockerContainer struct {
	ID        string      `json:"Id"`
	Name      string      `json:"Name"`
	Names     interface{} `json:"Names"` // Can be string or []string
	State     string      `json:"State"`
	Status    string      `json:"Status"`
	Image     string      `json:"Image"`
	LabelsRaw interface{} `json:"Labels"` // Can be string or map[string]string
	Ports     interface{} `json:"Ports"`  // Can be string or array
	Health    string      `json:"Health"`
}

// parseLabels parses the labels from various Docker formats
func (dc *dockerContainer) parseLabels() map[string]string {
	labels := make(map[string]string)

	switch v := dc.LabelsRaw.(type) {
	case string:
		// Parse comma-separated key=value pairs
		if v != "" {
			pairs := strings.Split(v, ",")
			for _, pair := range pairs {
				parts := strings.SplitN(pair, "=", 2)
				if len(parts) == 2 {
					labels[parts[0]] = parts[1]
				}
			}
		}
	case map[string]interface{}:
		// Convert from map[string]interface{} to map[string]string
		for k, val := range v {
			if str, ok := val.(string); ok {
				labels[k] = str
			}
		}
	}

	return labels
}

// getNames returns the container names as a slice
func (dc *dockerContainer) getNames() []string {
	switch v := dc.Names.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []interface{}:
		names := make([]string, 0, len(v))
		for _, n := range v {
			if str, ok := n.(string); ok {
				names = append(names, str)
			}
		}
		return names
	case []string:
		return v
	}
	return []string{}
}

// getPorts parses the ports from various Docker formats
func (dc *dockerContainer) getPorts() []PortMapping {
	var ports []PortMapping

	switch v := dc.Ports.(type) {
	case string:
		// Parse string format like "0.0.0.0:11434->11434/tcp, [::]:11434->11434/tcp"
		if v != "" {
			// Split by comma for multiple port mappings
			portMappings := strings.Split(v, ", ")
			for _, mapping := range portMappings {
				mapping = strings.TrimSpace(mapping)
				if mapping == "" {
					continue
				}

				// Skip IPv6 mappings for now (starting with [::])
				if strings.HasPrefix(mapping, "[::]:") {
					continue
				}

				// Parse format: "0.0.0.0:11434->11434/tcp"
				var hostIP, hostPort, containerPort, protocol string

				// Split by -> to separate host and container parts
				parts := strings.Split(mapping, "->")
				if len(parts) == 2 {
					// Parse host part (0.0.0.0:11434)
					hostPart := parts[0]
					if idx := strings.LastIndex(hostPart, ":"); idx != -1 {
						hostIP = hostPart[:idx]
						hostPort = hostPart[idx+1:]
					}

					// Parse container part (11434/tcp)
					containerPart := parts[1]
					if idx := strings.Index(containerPart, "/"); idx != -1 {
						containerPort = containerPart[:idx]
						protocol = containerPart[idx+1:]
					} else {
						containerPort = containerPart
						protocol = "tcp"
					}

					if hostPort != "" && containerPort != "" {
						ports = append(ports, PortMapping{
							HostIP:        hostIP,
							HostPort:      hostPort,
							ContainerPort: containerPort,
							Protocol:      protocol,
						})
					}
				}
			}
		}
	case []interface{}:
		for _, p := range v {
			if pm, ok := p.(map[string]interface{}); ok {
				port := PortMapping{}
				if hostIP, ok := pm["HostIp"].(string); ok {
					port.HostIP = hostIP
				}
				if hostPort, ok := pm["HostPort"].(string); ok {
					port.HostPort = hostPort
				} else if hostPort, ok := pm["HostPort"].(float64); ok {
					port.HostPort = fmt.Sprintf("%v", int(hostPort))
				}
				if containerPort, ok := pm["PrivatePort"].(float64); ok {
					port.ContainerPort = fmt.Sprintf("%v", int(containerPort))
				} else if containerPort, ok := pm["PrivatePort"].(string); ok {
					port.ContainerPort = containerPort
				}
				if proto, ok := pm["Type"].(string); ok {
					port.Protocol = proto
				}
				ports = append(ports, port)
			}
		}
	}

	return ports
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
	cmd := exec.CommandContext(ctx, s.dockerBinary, args...)
	cmd.Dir = absPath
	return cmd, nil
}

func (s *Service) listContainersForProject(ctx context.Context, project string) ([]ContainerSummary, error) {
	// Use Docker compose labels
	filters := []string{
		fmt.Sprintf("label=com.docker.compose.project=%s", project),
	}

	summaries, err := s.queryContainers(ctx, filters)
	if err != nil {
		return nil, err
	}

	return summaries, nil
}

func (s *Service) queryContainers(ctx context.Context, filters []string) ([]ContainerSummary, error) {
	args := []string{"ps", "--all"}
	for _, filter := range filters {
		args = append(args, "--filter", filter)
	}
	args = append(args, "--format", "json")

	// #nosec G204 -- arguments are generated internally for docker interaction
	cmd := exec.CommandContext(ctx, s.dockerBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []ContainerSummary{}, nil
	}

	// Docker outputs JSONL format (one JSON object per line)
	var containers []dockerContainer
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var container dockerContainer
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			return nil, fmt.Errorf("failed to parse docker container JSON: %w", err)
		}
		containers = append(containers, container)
	}

	summaries := make([]ContainerSummary, 0, len(containers))
	for _, cont := range containers {
		// Parse labels and get service name
		labels := cont.parseLabels()
		serviceName := labels["com.docker.compose.service"]

		// Get container name
		name := cont.Name
		names := cont.getNames()
		if name == "" && len(names) > 0 {
			name = names[0]
		}
		name = strings.TrimPrefix(name, "/")

		// Get ports
		ports := cont.getPorts()

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
	file, err := os.Open(envPath) //nolint:gosec // Path from compose directory
	if err != nil {
		return ""
	}
	defer file.Close() //nolint:errcheck // Cleanup, error not critical

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
