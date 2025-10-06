package runtime

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"gopkg.in/yaml.v3"
	"treeos/internal/naming"
)

// App represents a discovered application managed by Docker.
type App struct {
	Name           string                    `json:"name"`
	Path           string                    `json:"path"`
	Status         string                    `json:"status"`
	Services       map[string]ComposeService `json:"services,omitempty"`
	Error          string                    `json:"error,omitempty"`
	Emoji          string                    `json:"emoji,omitempty"`
	BypassSecurity bool                      `json:"bypassSecurity"`
}

// ComposeService represents an individual service definition from a compose file.
type ComposeService struct {
	Image       string   `json:"image" yaml:"image"`
	Ports       []string `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment []string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Volumes     []string `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}

// Compose models the sections of the compose file that TreeOS cares about.
type Compose struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
	XOnTree  *struct {
		Subdomain      string `yaml:"subdomain,omitempty"`
		HostPort       int    `yaml:"host_port,omitempty"`
		IsExposed      bool   `yaml:"is_exposed"`
		Emoji          string `yaml:"emoji,omitempty"`
		BypassSecurity bool   `yaml:"bypass_security"`
	} `yaml:"x-ontree,omitempty"`
}

// dockerContainer describes the subset of fields we need from Docker's container.Container.
type dockerContainer struct {
	ID     string
	Names  []string
	State  string
	Status string
	Labels map[string]string
}

// ScanApps iterates the apps directory to enumerate container apps and their Docker state.
func (c *Client) ScanApps(appsDir string) ([]*App, error) {
	files, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, err
	}

	containers, err := c.listContainers(context.Background())
	if err != nil {
		// Log the error but continue - apps can exist without containers running
		containers = []dockerContainer{}
	}

	apps := []*App{}
	for _, file := range files {
		if !file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		appPath := filepath.Join(appsDir, file.Name())
		composeFile := filepath.Join(appPath, "docker-compose.yml")
		if _, err := os.Stat(composeFile); err != nil {
			continue // Skip directories without docker-compose.yml
		}

		app := &App{
			Name:   file.Name(),
			Path:   appPath,
			Status: "unknown",
		}

		// Read compose metadata
		if compose, err := readComposeFile(composeFile); err == nil {
			app.Services = compose.Services
			if compose.XOnTree != nil {
				app.Emoji = compose.XOnTree.Emoji
				app.BypassSecurity = compose.XOnTree.BypassSecurity
			}
		}

		// Calculate app status from container states
		app.Status = c.getContainerStatus(app, containers)

		apps = append(apps, app)
	}

	return apps, nil
}

// GetAppDetails provides detailed information about a single app including compose metadata.
func (c *Client) GetAppDetails(appsDir, appName string) (*App, error) {
	appPath := filepath.Join(appsDir, appName)
	composeFile := filepath.Join(appPath, "docker-compose.yml")

	if _, err := os.Stat(composeFile); err != nil {
		return nil, fmt.Errorf("compose file not found: %w", err)
	}

	compose, err := readComposeFile(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	app := &App{
		Name:     appName,
		Path:     appPath,
		Services: compose.Services,
		Status:   "unknown",
	}

	if compose.XOnTree != nil {
		app.Emoji = compose.XOnTree.Emoji
		app.BypassSecurity = compose.XOnTree.BypassSecurity
	}

	// Get container status for this specific app
	containers, err := c.listContainers(context.Background())
	if err == nil {
		app.Status = c.getContainerStatus(app, containers)
	}

	return app, nil
}

// getContainerStatus determines the overall status of an app based on its containers.
func (c *Client) getContainerStatus(app *App, containers []dockerContainer) string {
	candidates := projectNameCandidates(app)
	runningCount := 0
	exitedCount := 0
	totalCount := 0

	for _, container := range containers {
		if containerMatchesProject(container, candidates) {
			totalCount++
			switch container.State {
			case "running":
				runningCount++
			case "exited", "stopped":
				exitedCount++
			}
		}
	}

	if totalCount == 0 {
		return "not created"
	}
	if runningCount == totalCount {
		return "running"
	}
	if exitedCount == totalCount {
		return "exited"
	}
	if runningCount > 0 {
		return "partial"
	}

	return "unknown"
}

// listContainers returns Docker containers relevant to TreeOS apps.
func (c *Client) listContainers(ctx context.Context) ([]dockerContainer, error) {
	if c.dockerClient == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list Docker containers: %w", err)
	}

	result := make([]dockerContainer, 0, len(containers))
	for _, cnt := range containers {
		result = append(result, dockerContainer{
			ID:     cnt.ID,
			Names:  cnt.Names,
			State:  cnt.State,
			Status: cnt.Status,
			Labels: cnt.Labels,
		})
	}

	return result, nil
}

// containerMatchesProject checks if a container belongs to a specific project.
func containerMatchesProject(container dockerContainer, candidates []string) bool {
	// Check by compose project label first (most reliable)
	if projectLabel, ok := container.Labels["com.docker.compose.project"]; ok {
		for _, candidate := range candidates {
			if strings.EqualFold(projectLabel, candidate) {
				return true
			}
		}
	}

	// Fall back to name matching for containers without labels
	for _, name := range container.Names {
		name = strings.TrimPrefix(name, "/")
		for _, candidate := range candidates {
			if strings.HasPrefix(strings.ToLower(name), strings.ToLower(candidate)+"-") {
				return true
			}
		}
	}

	return false
}

// projectNameCandidates returns possible Docker Compose project names for an app.
func projectNameCandidates(app *App) []string {
	// Include standard Docker naming patterns
	appIdentifier := naming.GetAppIdentifier(app.Path)

	candidates := []string{
		appIdentifier,
		naming.GetComposeProjectName(appIdentifier),
		"ontree-" + appIdentifier,
		app.Name,
		strings.ToLower(app.Name),
	}

	// Deduplicate
	seen := map[string]struct{}{}
	result := []string{}
	for _, candidate := range candidates {
		if candidate != "" {
			if _, ok := seen[strings.ToLower(candidate)]; !ok {
				result = append(result, candidate)
				seen[strings.ToLower(candidate)] = struct{}{}
			}
		}
	}

	return result
}

// readComposeFile parses a docker-compose.yml file.
func readComposeFile(path string) (*Compose, error) {
	file, err := os.Open(path) //nolint:gosec // Path from trusted app directory listing
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // Cleanup, error not critical

	// Use bufio for efficient reading
	reader := bufio.NewReader(file)
	var compose Compose
	if err := yaml.NewDecoder(reader).Decode(&compose); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	return &compose, nil
}