// Package systemcheck provides system health checks for TreeOS dependencies and configuration.
package systemcheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"treeos/internal/config"
)

// Status represents the health status of a system check.
type Status string

const (
	// StatusOK indicates the check passed successfully.
	StatusOK Status = "ok"
	// StatusError indicates the check failed.
	StatusError Status = "error"
)

// CheckResult represents the result of a single system check.
type CheckResult struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      Status   `json:"status"`
	Message     string   `json:"message"`
	Version     string   `json:"version,omitempty"`
	Details     string   `json:"details,omitempty"`
	Remediation []string `json:"remediation,omitempty"`
}

// Runner executes system health checks.
type Runner struct {
	cfg *config.Config
}

// NewRunner creates a new system check runner with the provided configuration.
func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run executes all system checks and returns the results.
func (r *Runner) Run(ctx context.Context) []CheckResult {
	return []CheckResult{
		r.checkDirectories(),
		r.checkDocker(ctx),
		r.checkDockerCompose(ctx),
		r.checkCaddy(ctx),
	}
}

func (r *Runner) checkDirectories() CheckResult {
	shared := sharedPath(r.cfg)
	paths := []string{
		r.cfg.AppsDir,
		filepath.Join(r.cfg.AppsDir, "mount"),
		shared,
		filepath.Join(shared, "ollama"),
		logsPath(r.cfg),
	}

	seen := make(map[string]struct{})
	created := make([]string, 0, len(paths))

	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, exists := seen[p]; exists {
			continue
		}

		if err := os.MkdirAll(p, 0o755); err != nil { //nolint:gosec // Directory permissions appropriate
			return CheckResult{
				ID:          "directories",
				Name:        "Prepare system directories",
				Status:      StatusError,
				Message:     fmt.Sprintf("Failed to prepare %s", p),
				Details:     err.Error(),
				Remediation: directoryRemediation(p),
			}
		}
		seen[p] = struct{}{}
		created = append(created, p)
	}

	return CheckResult{
		ID:      "directories",
		Name:    "Prepare system directories",
		Status:  StatusOK,
		Message: "System directories are ready",
		Details: strings.Join(created, "\n"),
	}
}

func (r *Runner) checkDocker(ctx context.Context) CheckResult {
	version, err := commandVersion(ctx, "docker", "--version")
	if err != nil {
		return CheckResult{
			ID:          "docker",
			Name:        "Docker",
			Status:      StatusError,
			Message:     "Docker not available",
			Details:     err.Error(),
			Remediation: dockerRemediation(),
		}
	}

	// Test Docker daemon connection
	if _, err := commandOutput(ctx, "docker", "info"); err != nil {
		return CheckResult{
			ID:          "docker",
			Name:        "Docker",
			Status:      StatusError,
			Message:     "Docker daemon not reachable",
			Details:     err.Error(),
			Remediation: dockerDaemonRemediation(),
		}
	}

	return CheckResult{
		ID:      "docker",
		Name:    "Docker",
		Status:  StatusOK,
		Message: "Docker detected and running",
		Version: version,
	}
}

func (r *Runner) checkDockerCompose(ctx context.Context) CheckResult {
	// Try docker compose (v2)
	version, err := commandVersion(ctx, "docker", "compose", "version")
	if err == nil {
		return CheckResult{
			ID:      "docker_compose",
			Name:    "Docker Compose",
			Status:  StatusOK,
			Message: "Docker Compose v2 ready",
			Version: version,
		}
	}

	// Try docker-compose (v1)
	version, err = commandVersion(ctx, "docker-compose", "--version")
	if err == nil {
		return CheckResult{
			ID:      "docker_compose",
			Name:    "Docker Compose",
			Status:  StatusOK,
			Message: "Docker Compose v1 ready",
			Version: version,
		}
	}

	return CheckResult{
		ID:          "docker_compose",
		Name:        "Docker Compose",
		Status:      StatusError,
		Message:     "Docker Compose not available",
		Details:     "Neither 'docker compose' (v2) nor 'docker-compose' (v1) found",
		Remediation: dockerComposeRemediation(),
	}
}

func (r *Runner) checkCaddy(ctx context.Context) CheckResult {
	version, err := commandVersion(ctx, "caddy", "version")
	if err != nil {
		return CheckResult{
			ID:          "caddy",
			Name:        "Caddy",
			Status:      StatusError,
			Message:     "Caddy not available",
			Details:     err.Error(),
			Remediation: caddyRemediation(),
		}
	}

	return CheckResult{
		ID:      "caddy",
		Name:    "Caddy",
		Status:  StatusOK,
		Message: "Caddy detected",
		Version: version,
	}
}

func sharedPath(_ *config.Config) string {
	return config.GetSharedPath()
}

func logsPath(cfg *config.Config) string {
	base := filepath.Dir(cfg.DatabasePath)
	if base == "" || base == "." {
		base = "."
	}
	return filepath.Join(base, "logs")
}

func commandVersion(ctx context.Context, binary string, args ...string) (string, error) {
	return commandOutput(ctx, binary, args...)
}

func commandOutput(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func directoryRemediation(path string) []string {
	return []string{
		fmt.Sprintf("Create the directory: sudo mkdir -p %s", path),
		fmt.Sprintf("Set permissions: sudo chmod 755 %s", path),
		fmt.Sprintf("Set ownership: sudo chown $USER %s", path),
	}
}

func dockerRemediation() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"Install Docker Desktop from https://docker.com/products/docker-desktop",
			"Start Docker Desktop from Applications",
		}
	case "linux":
		return []string{
			"Install Docker: curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh",
			"Add user to docker group: sudo usermod -aG docker $USER",
			"Start Docker service: sudo systemctl start docker",
			"Enable Docker service: sudo systemctl enable docker",
			"Log out and back in for group changes to take effect",
		}
	default:
		return []string{
			"Install Docker from https://docker.com",
		}
	}
}

func dockerDaemonRemediation() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"Ensure Docker Desktop is running",
			"Check Docker Desktop settings",
			"Restart Docker Desktop if needed",
		}
	case "linux":
		return []string{
			"Start Docker service: sudo systemctl start docker",
			"Check service status: sudo systemctl status docker",
			"Check Docker logs: sudo journalctl -u docker",
			"Ensure user is in docker group: groups $USER",
		}
	default:
		return []string{
			"Ensure Docker daemon is running",
			"Check Docker service status",
		}
	}
}

func dockerComposeRemediation() []string {
	return []string{
		"Install Docker Compose v2 (recommended): Docker Desktop includes it",
		"Or install standalone: sudo apt-get install docker-compose-plugin",
		"Or download binary: https://github.com/docker/compose/releases",
	}
}

func caddyRemediation() []string {
	return []string{
		"Install Caddy: sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl",
		"Add Caddy repo: curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
		"Install: sudo apt update && sudo apt install caddy",
		"Or download from: https://caddyserver.com/download",
	}
}