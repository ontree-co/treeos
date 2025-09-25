package systemcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"treeos/internal/config"
)

type Status string

const (
	StatusOK    Status = "ok"
	StatusError Status = "error"
)

type CheckResult struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      Status   `json:"status"`
	Message     string   `json:"message"`
	Version     string   `json:"version,omitempty"`
	Details     string   `json:"details,omitempty"`
	Remediation []string `json:"remediation,omitempty"`
}

type Runner struct {
	cfg *config.Config
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) Run(ctx context.Context) []CheckResult {
	return []CheckResult{
		r.checkDirectories(),
		r.checkPodman(ctx),
		r.checkPodmanService(ctx),
		r.checkPodmanCompose(ctx),
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

		if err := os.MkdirAll(p, 0o755); err != nil {
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

func (r *Runner) checkPodman(ctx context.Context) CheckResult {
	version, err := commandVersion(ctx, "podman", "--version")
	if err != nil {
		return CheckResult{
			ID:          "podman",
			Name:        "Podman",
			Status:      StatusError,
			Message:     "Podman client not available",
			Details:     err.Error(),
			Remediation: binaryRemediation("podman"),
		}
	}

	return CheckResult{
		ID:      "podman",
		Name:    "Podman",
		Status:  StatusOK,
		Message: "Podman detected",
		Version: version,
	}
}

func (r *Runner) checkPodmanService(ctx context.Context) CheckResult {
	output, err := commandOutput(ctx, "podman", "info", "--format", "{{json .Host.RemoteSocket}}")
	if err != nil {
		return CheckResult{
			ID:          "podman_service",
			Name:        "Podman Service",
			Status:      StatusError,
			Message:     "Podman service not reachable",
			Details:     err.Error(),
			Remediation: podmanServiceRemediation(""),
		}
	}

	var remote struct {
		Path   string `json:"Path"`
		Exists bool   `json:"Exists"`
	}

	if err := json.Unmarshal([]byte(output), &remote); err != nil {
		return CheckResult{
			ID:      "podman_service",
			Name:    "Podman Service",
			Status:  StatusOK,
			Message: "Podman service reachable",
			Details: strings.TrimSpace(output),
		}
	}

	if remote.Path != "" && !remote.Exists {
		return CheckResult{
			ID:          "podman_service",
			Name:        "Podman Service",
			Status:      StatusError,
			Message:     "Podman service socket not active",
			Details:     fmt.Sprintf("Socket %s not listening", remote.Path),
			Remediation: podmanServiceRemediation(remote.Path),
		}
	}

	message := "Podman service reachable"
	if remote.Path != "" {
		message = fmt.Sprintf("Remote socket ready (%s)", remote.Path)
	}

	return CheckResult{
		ID:      "podman_service",
		Name:    "Podman Service",
		Status:  StatusOK,
		Message: message,
		Details: remote.Path,
	}
}

func (r *Runner) checkPodmanCompose(ctx context.Context) CheckResult {
	var errs []string

	if version, err := commandVersion(ctx, "podman", "compose", "version"); err == nil {
		return CheckResult{
			ID:      "podman_compose",
			Name:    "Podman Compose",
			Status:  StatusOK,
			Message: "podman compose detected",
			Version: version,
		}
	} else if err != nil {
		errs = append(errs, err.Error())
	}

	if version, err := commandVersion(ctx, "podman-compose", "--version"); err == nil {
		return CheckResult{
			ID:      "podman_compose",
			Name:    "Podman Compose",
			Status:  StatusOK,
			Message: "podman-compose detected",
			Version: version,
		}
	} else if err != nil {
		errs = append(errs, err.Error())
	}

	details := strings.Join(errs, "; ")
	return CheckResult{
		ID:          "podman_compose",
		Name:        "Podman Compose",
		Status:      StatusError,
		Message:     "Podman compose not available",
		Details:     details,
		Remediation: binaryRemediation("podman-compose"),
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
			Remediation: binaryRemediation("caddy"),
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

func sharedPath(cfg *config.Config) string {
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

func commandOutput(ctx context.Context, binary string, args ...string) (string, error) {
	if _, err := exec.LookPath(binary); err != nil {
		return "", err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	// #nosec G204 -- commands use static arguments
	cmd := exec.CommandContext(timeoutCtx, binary, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("%w (output: %s)", err, trimmed)
	}

	return trimmed, nil
}

func directoryRemediation(path string) []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			fmt.Sprintf("Ensure write access: sudo chown -R $USER:$USER %s", path),
			fmt.Sprintf("Adjust permissions if needed: sudo chmod 755 %s", path),
		}
	case "darwin":
		return []string{
			fmt.Sprintf("Ensure write access: sudo chown -R $USER %s", path),
		}
	default:
		return []string{"Ensure the TreeOS process can write to " + path}
	}
}

func binaryRemediation(binary string) []string {
	switch runtime.GOOS {
	case "linux":
		switch binary {
		case "podman":
			return []string{
				"Install Podman: sudo apt install podman",
				"After installation, verify with 'podman info'",
				"See https://podman.io/docs/installation",
			}
		case "podman-compose":
			return []string{
				"Install podman-compose: sudo apt install podman-compose",
				"Alternatively, Podman 4+ includes podman compose",
			}
		case "caddy":
			return []string{
				"Install Caddy: sudo apt install caddy",
				"See https://caddyserver.com/docs/install",
			}
		}
	case "darwin":
		switch binary {
		case "podman":
			return []string{
				"Install Podman: brew install podman",
				"Initialize and start: podman machine init && podman machine start",
			}
		case "podman-compose":
			return []string{
				"Install podman-compose: brew install podman-compose",
				"Podman 4+ also provides 'podman compose'",
			}
		case "caddy":
			return []string{
				"Install Caddy: brew install caddy",
			}
		}
	}

	return []string{fmt.Sprintf("Install %s and ensure it is on PATH", binary)}
}

func podmanServiceRemediation(socketPath string) []string {
	switch runtime.GOOS {
	case "linux":
		steps := []string{
			"Start rootless Podman socket: systemctl --user enable --now podman.socket",
			"Verify with: systemctl --user status podman.socket",
		}
		if socketPath != "" {
			steps = append(steps, fmt.Sprintf("Expected socket: %s", socketPath))
		}
		return steps
	case "darwin":
		return []string{
			"Ensure Podman machine is running: podman machine start",
			"If needed, restart the machine: podman machine stop && podman machine start",
		}
	}

	return []string{"Ensure the Podman service/socket is active"}
}
