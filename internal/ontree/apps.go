package ontree

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ontree-co/treeos/internal/config"
	"github.com/ontree-co/treeos/internal/yamlutil"
	"github.com/ontree-co/treeos/pkg/compose"
	"gopkg.in/yaml.v3"
)

var appIDRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

// AppInstall installs an app template and prepares its files.
func (m *Manager) AppInstall(ctx context.Context, appID, version, envPath string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 1)
	go func() {
		defer close(ch)
		if appID == "" || !appIDRegex.MatchString(appID) {
			ch <- ProgressEvent{Type: "error", Message: "invalid app id", Code: "invalid_app_id"}
			return
		}

		template, err := m.templateSvc.GetTemplateByID(appID)
		if err != nil {
			ch <- ProgressEvent{Type: "error", Message: "app template not found", Code: "app_not_found"}
			return
		}

		content, err := m.templateSvc.GetTemplateContent(template)
		if err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "template_read_failed"}
			return
		}
		content = m.templateSvc.ProcessTemplateContent(content, appID)

		var envContent string
		if envPath != "" {
			raw, err := os.ReadFile(envPath)
			if err != nil {
				ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "env_read_failed"}
				return
			}
			envContent = string(raw)
		}

		envContent = m.injectOpenWebUIAdminEnv(appID, envContent)

		if err := m.createAppScaffoldFromTemplate(appID, content, envContent, ""); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "app_create_failed"}
			return
		}

		ch <- ProgressEvent{Type: "success", Message: "app installed"}
	}()
	return ch
}

// AppStart starts containers for an app.
func (m *Manager) AppStart(ctx context.Context, appID string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 1)
	go func() {
		defer close(ch)
		if err := m.ensureCompose(); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "compose_unavailable"}
			return
		}

		appPath := filepath.Join(m.cfg.AppsDir, appID)
		opts := compose.Options{WorkingDir: appPath}
		if _, err := os.Stat(filepath.Join(appPath, ".env")); err == nil {
			opts.EnvFile = ".env"
		}

		progress := func(line string) {
			ch <- ProgressEvent{Type: "log", Message: line}
		}

		if err := m.composeSvc.UpWithProgress(ctx, opts, progress); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "compose_error"}
			return
		}

		ch <- ProgressEvent{Type: "success", Message: "app started"}
	}()
	return ch
}

// AppStop stops containers for an app.
func (m *Manager) AppStop(ctx context.Context, appID string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 1)
	go func() {
		defer close(ch)
		if err := m.ensureCompose(); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "compose_unavailable"}
			return
		}

		appPath := filepath.Join(m.cfg.AppsDir, appID)
		opts := compose.Options{WorkingDir: appPath}
		if _, err := os.Stat(filepath.Join(appPath, ".env")); err == nil {
			opts.EnvFile = ".env"
		}

		if err := m.composeSvc.Down(ctx, opts, false); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "compose_error"}
			return
		}

		ch <- ProgressEvent{Type: "success", Message: "app stopped"}
	}()
	return ch
}

// AppHealth checks app containers and optional HTTP readiness.
func (m *Manager) AppHealth(ctx context.Context, appID, httpURL string, timeout, interval time.Duration) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 1)
	go func() {
		defer close(ch)
		if err := m.ensureCompose(); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "compose_unavailable"}
			return
		}
		if timeout <= 0 {
			timeout = 180 * time.Second
		}
		if interval <= 0 {
			interval = 3 * time.Second
		}

		appPath := filepath.Join(m.cfg.AppsDir, appID)
		opts := compose.Options{WorkingDir: appPath}
		if _, err := os.Stat(filepath.Join(appPath, ".env")); err == nil {
			opts.EnvFile = ".env"
		}

		deadline := time.Now().Add(timeout)
		for {
			select {
			case <-ctx.Done():
				ch <- ProgressEvent{Type: "error", Message: ctx.Err().Error(), Code: "context_cancelled"}
				return
			default:
			}

			ok, err := m.checkContainersHealthy(ctx, opts)
			if err != nil {
				ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "health_check_failed"}
				return
			}

			if ok && (httpURL == "" || m.checkHTTPReady(ctx, httpURL)) {
				ch <- ProgressEvent{Type: "success", Message: "app healthy"}
				return
			}

			if time.Now().After(deadline) {
				ch <- ProgressEvent{Type: "error", Message: "health check timeout", Code: "health_timeout"}
				return
			}
			time.Sleep(interval)
		}
	}()
	return ch
}

// AppList lists apps by scanning the apps directory.
func (m *Manager) AppList(ctx context.Context) ([]App, error) {
	entries, err := os.ReadDir(m.cfg.AppsDir)
	if err != nil {
		return nil, err
	}

	apps := make([]App, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		appPath := filepath.Join(m.cfg.AppsDir, entry.Name())
		if _, err := os.Stat(filepath.Join(appPath, "docker-compose.yml")); err != nil {
			continue
		}
		apps = append(apps, App{ID: entry.Name(), Name: entry.Name()})
	}

	return apps, nil
}

func (m *Manager) ensureCompose() error {
	if m.composeSvc != nil {
		return nil
	}
	svc, err := compose.NewService()
	if err != nil {
		return err
	}
	m.composeSvc = svc
	return nil
}

func (m *Manager) checkContainersHealthy(ctx context.Context, opts compose.Options) (bool, error) {
	containers, err := m.composeSvc.PS(ctx, opts)
	if err != nil {
		return false, err
	}
	if len(containers) == 0 {
		return false, nil
	}

	for _, container := range containers {
		if container.State != "running" {
			return false, nil
		}
		if container.Health != "" && container.Health != "healthy" {
			return false, nil
		}
	}

	return true, nil
}

func (m *Manager) checkHTTPReady(ctx context.Context, httpURL string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURL, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func (m *Manager) createAppScaffoldFromTemplate(appName, composeContent, envContent, emoji string) error {
	appPath := filepath.Join(m.cfg.AppsDir, appName)

	if err := m.createAppScaffoldInternal(appPath, appName, composeContent, envContent, emoji); err != nil {
		return err
	}

	if err := m.generateAppYamlWithFlags(appPath, appName, composeContent, true); err != nil {
		return err
	}

	return nil
}

func (m *Manager) createAppScaffoldInternal(appPath, appName, composeContent, envContent, emoji string) error {
	if err := os.MkdirAll(appPath, 0750); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	if usesSharedModels(composeContent) {
		if err := ensureSharedModelsDirectory(); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Join(appPath, "mnt"), 0750); err != nil {
		return fmt.Errorf("failed to create mnt directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "volumes"), 0750); err != nil {
		return fmt.Errorf("failed to create volumes directory: %w", err)
	}

	composePath := filepath.Join(appPath, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0600); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	namingConfig := fmt.Sprintf("COMPOSE_PROJECT_NAME=ontree-%s\nCOMPOSE_SEPARATOR=-\n", strings.ToLower(appName))
	if envContent != "" {
		if !strings.Contains(envContent, "COMPOSE_PROJECT_NAME=") {
			envContent = namingConfig + envContent
		}
	} else {
		envContent = namingConfig
	}
	envPath := filepath.Join(appPath, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	hostPort, err := extractHostPort(composeContent)
	if err != nil {
		hostPort = 0
	}
	if err := m.addMetadata(composePath, appName, emoji, hostPort); err != nil {
		return err
	}

	return nil
}

func (m *Manager) addMetadata(composePath, appName, emoji string, hostPort int) error {
	yamlData, err := yamlutil.ReadComposeWithMetadata(composePath)
	if err != nil {
		return err
	}
	metadata := &yamlutil.OnTreeMetadata{
		Subdomain: appName,
		HostPort:  hostPort,
		IsExposed: false,
		Emoji:     emoji,
	}
	yamlutil.SetOnTreeMetadata(yamlData, metadata)
	if err := yamlutil.WriteComposeWithMetadata(composePath, yamlData); err != nil {
		return err
	}
	return nil
}

func (m *Manager) generateAppYamlWithFlags(appPath, appName, composeContent string, fromTemplate bool) error {
	var composeFile map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &composeFile); err != nil {
		return fmt.Errorf("failed to parse docker-compose.yml: %w", err)
	}

	services := []string{}
	primaryService := ""
	if servicesMap, ok := composeFile["services"].(map[string]interface{}); ok {
		for serviceName := range servicesMap {
			services = append(services, serviceName)
			if primaryService == "" {
				primaryService = serviceName
			}
		}
	}

	appConfig := map[string]interface{}{
		"id":                strings.ToLower(appName),
		"name":              strings.ToLower(appName),
		"primary_service":   primaryService,
		"expected_services": services,
	}
	if fromTemplate {
		appConfig["initial_setup_required"] = true
	}
	if strings.Contains(strings.ToLower(appName), "uptime") {
		appConfig["uptime_kuma_monitor"] = ""
	}

	appYmlData, err := yaml.Marshal(appConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal app config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "app.yml"), appYmlData, 0600); err != nil {
		return fmt.Errorf("failed to write app.yml: %w", err)
	}
	return nil
}

func extractHostPort(composeContent string) (int, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &data); err != nil {
		return 0, fmt.Errorf("failed to parse YAML: %w", err)
	}

	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("no services found in docker-compose")
	}

	for _, service := range services {
		serviceMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}
		ports, ok := serviceMap["ports"]
		if !ok {
			continue
		}
		portsSlice, ok := ports.([]interface{})
		if !ok || len(portsSlice) == 0 {
			continue
		}
		portStr, ok := portsSlice[0].(string)
		if !ok {
			continue
		}
		parts := strings.Split(portStr, ":")
		if len(parts) >= 1 {
			var hostPort int
			if _, err := fmt.Sscanf(parts[0], "%d", &hostPort); err == nil && hostPort > 0 {
				return hostPort, nil
			}
		}
	}

	return 0, fmt.Errorf("no host port found in docker-compose")
}

func usesSharedModels(composeContent string) bool {
	sharedPath := config.GetSharedPath()
	sharedOllamaPath := config.GetSharedOllamaPath()

	return strings.Contains(composeContent, sharedOllamaPath) ||
		strings.Contains(composeContent, "./shared/ollama") ||
		strings.Contains(composeContent, "../../shared/ollama") ||
		strings.Contains(composeContent, sharedPath) ||
		strings.Contains(composeContent, "./shared/") ||
		strings.Contains(composeContent, "../../shared/")
}

func ensureSharedModelsDirectory() error {
	sharedModelsDir := config.GetSharedOllamaPath()
	if sharedModelsDir == "" {
		return errors.New("shared models directory is not configured")
	}
	return os.MkdirAll(sharedModelsDir, 0750)
}

// Security validation can be added when we wire CLI start to policy enforcement.

func (m *Manager) injectOpenWebUIAdminEnv(appID, envContent string) string {
	if appID != "openwebui" {
		return envContent
	}

	email := os.Getenv("TREEOS_OPENWEBUI_ADMIN_EMAIL")
	password := os.Getenv("TREEOS_OPENWEBUI_ADMIN_PASSWORD")
	name := os.Getenv("TREEOS_OPENWEBUI_ADMIN_NAME")
	if email == "" || password == "" {
		return envContent
	}
	if name == "" {
		name = "Admin"
	}

	envContent = ensureEnvLine(envContent, "WEBUI_ADMIN_EMAIL", email)
	envContent = ensureEnvLine(envContent, "WEBUI_ADMIN_PASSWORD", password)
	envContent = ensureEnvLine(envContent, "WEBUI_ADMIN_NAME", name)

	return envContent
}

func ensureEnvLine(envContent, key, value string) string {
	if strings.Contains(envContent, key+"=") {
		return envContent
	}
	if envContent != "" && !strings.HasSuffix(envContent, "\n") {
		envContent += "\n"
	}
	envContent += fmt.Sprintf("%s=%s\n", key, value)
	return envContent
}
