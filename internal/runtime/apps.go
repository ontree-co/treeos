package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"treeos/internal/naming"
)

// App represents a discovered application managed by the container runtime.
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

// podmanContainer describes the subset of fields returned by `podman ps --format json` that
// we need in order to understand container state.
type podmanContainer struct {
	ID     string            `json:"Id"`
	Name   string            `json:"Name"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

// ScanApps loads compose metadata for every application and annotates it with container status.
func (c *Client) ScanApps(appsDir string) ([]*App, error) {
	var apps []*App

	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		return apps, nil
	}

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read apps directory: %w", err)
	}

	ctx := context.Background()
	containers, err := c.listContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list podman containers: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appPath := filepath.Join(appsDir, entry.Name())
		composePath := filepath.Join(appPath, "docker-compose.yml")
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			continue
		}

		app := &App{
			Name: entry.Name(),
			Path: appPath,
		}

		services, emoji, bypassSecurity, err := parseComposeFile(composePath)
		if err != nil {
			app.Status = "error"
			app.Error = fmt.Sprintf("failed to parse compose file: %v", err)
		} else {
			app.Services = services
			app.Emoji = emoji
			app.BypassSecurity = bypassSecurity
			app.Status = c.getContainerStatus(app, containers)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// GetAppDetails reads compose metadata for a specific application and attaches runtime status.
func (c *Client) GetAppDetails(appsDir, appName string) (*App, error) {
	appPath := filepath.Join(appsDir, appName)
	composePath := filepath.Join(appPath, "docker-compose.yml")

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("application not found: %s", appName)
	}

	services, emoji, bypassSecurity, err := parseComposeFile(composePath)
	if err != nil {
		return &App{
			Name:   appName,
			Path:   appPath,
			Status: "error",
			Error:  fmt.Sprintf("failed to parse compose file: %v", err),
		}, nil
	}

	ctx := context.Background()
	containers, err := c.listContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list podman containers: %w", err)
	}

	return &App{
		Name:           appName,
		Path:           appPath,
		Services:       services,
		Emoji:          emoji,
		BypassSecurity: bypassSecurity,
		Status:         c.getContainerStatus(&App{Name: appName, Path: appPath}, containers),
	}, nil
}

// listContainers retrieves all containers (running and stopped) from Podman.
func (c *Client) listContainers(ctx context.Context) ([]podmanContainer, error) {
	output, err := c.run(ctx, "ps", "--all", "--format", "json")
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []podmanContainer{}, nil
	}

	var containers []podmanContainer
	if err := json.Unmarshal([]byte(trimmed), &containers); err != nil {
		return nil, fmt.Errorf("failed to parse podman ps output: %w", err)
	}

	return containers, nil
}

// getContainerStatus determines the aggregated status for the given application.
func (c *Client) getContainerStatus(app *App, containers []podmanContainer) string {
	candidates := projectNameCandidates(app)

	running := 0
	other := 0

	for _, cont := range containers {
		if containerMatchesProject(cont, candidates) {
			switch strings.ToLower(cont.State) {
			case "running":
				running++
			case "configured", "created", "stopped", "exited", "paused", "stoppedwitherror":
				other++
			default:
				other++
			}
		}
	}

	if running > 0 && other > 0 {
		return "partial"
	}
	if running > 0 {
		return "running"
	}
	if other > 0 {
		return "exited"
	}

	return "not_created"
}

// parseComposeFile parses docker-compose.yml style files and extracts metadata.
func parseComposeFile(path string) (map[string]ComposeService, string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", false, err
	}

	var compose Compose
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, "", false, err
	}

	emoji := ""
	bypass := false
	if compose.XOnTree != nil {
		emoji = compose.XOnTree.Emoji
		bypass = compose.XOnTree.BypassSecurity
	}

	return compose.Services, emoji, bypass, nil
}

// projectNameCandidates returns possible compose project names for the application.
func projectNameCandidates(app *App) []string {
	candidates := []string{}

	addCandidate := func(val string) {
		if val == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, val) {
				return
			}
		}
		candidates = append(candidates, val)
	}

	base := filepath.Base(app.Path)
	addCandidate(base)
	addCandidate(strings.ToLower(base))
	addCandidate(sanitizeProjectName(base))
	addCandidate(app.Name)
	addCandidate(strings.ToLower(app.Name))
	addCandidate(sanitizeProjectName(app.Name))

	// Include naming helpers used by the system for deterministic project IDs
	appIdentifier := naming.GetAppIdentifier(app.Path)
	addCandidate(appIdentifier)
	addCandidate(strings.ToLower(appIdentifier))
	addCandidate(sanitizeProjectName(appIdentifier))

	projectName := naming.GetComposeProjectName(appIdentifier)
	addCandidate(projectName)
	addCandidate(strings.ToLower(projectName))
	addCandidate(sanitizeProjectName(projectName))

	if envProject := projectNameFromEnv(app.Path); envProject != "" {
		addCandidate(envProject)
		addCandidate(strings.ToLower(envProject))
		addCandidate(sanitizeProjectName(envProject))
	}

	return candidates
}

// projectNameFromEnv looks for COMPOSE_PROJECT_NAME in the app's .env file.
func projectNameFromEnv(appPath string) string {
	envPath := filepath.Join(appPath, ".env")
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
			return strings.TrimSpace(strings.TrimPrefix(line, "COMPOSE_PROJECT_NAME="))
		}
	}

	return ""
}

// sanitizeProjectName mirrors Docker/Podman's normalisation rules for compose projects.
func sanitizeProjectName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)

	var b strings.Builder
	prevHyphen := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}

		if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}

	sanitized := b.String()
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return name
	}
	return sanitized
}

// containerMatchesProject checks whether the container belongs to any of the candidate project names.
func containerMatchesProject(cont podmanContainer, candidates []string) bool {
	labels := cont.Labels

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		lowerCandidate := strings.ToLower(candidate)
		dashCandidate := sanitizeProjectName(candidate)
		underscoreCandidate := strings.ReplaceAll(dashCandidate, "-", "_")

		if project := labels["io.podman.compose.project"]; project != "" && strings.EqualFold(project, candidate) {
			return true
		}

		names := cont.Names
		if len(names) == 0 && cont.Name != "" {
			names = []string{cont.Name}
		}

		for _, name := range names {
			clean := strings.TrimPrefix(name, "/")
			cleanLower := strings.ToLower(clean)

			if strings.HasPrefix(cleanLower, lowerCandidate+"-") || strings.HasPrefix(cleanLower, lowerCandidate+"_") {
				return true
			}
			if dashCandidate != "" && (strings.HasPrefix(cleanLower, dashCandidate+"-") || strings.HasPrefix(cleanLower, dashCandidate+"_")) {
				return true
			}
			if underscoreCandidate != "" && (strings.HasPrefix(cleanLower, underscoreCandidate+"-") || strings.HasPrefix(cleanLower, underscoreCandidate+"_")) {
				return true
			}
		}
	}

	return false
}
