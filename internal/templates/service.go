// Package templates provides template management functionality for application templates
package templates

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"treeos/internal/config"
	"treeos/internal/embeds"
)

// Template represents an application template
type Template struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Icon             string `json:"icon"`
	Filename         string `json:"filename"`
	Port             string `json:"port"`
	DocumentationURL string `json:"documentation_url"`
	IsSystemService  bool   `json:"is_system_service,omitempty"`
}

// File represents the structure of the templates.json file
type File struct {
	Templates []Template `json:"templates"`
}

// Service provides template management functionality
type Service struct {
	templatesPath string
}

// NewService creates a new template service instance
func NewService(templatesPath string) *Service {
	return &Service{
		templatesPath: templatesPath,
	}
}

// GetAvailableTemplates returns all available application templates
func (s *Service) GetAvailableTemplates() ([]Template, error) {
	templateFS, err := embeds.TemplateFS()
	if err != nil {
		return nil, fmt.Errorf("failed to get template filesystem: %w", err)
	}

	jsonPath := filepath.Join(s.templatesPath, "templates.json")
	fmt.Printf("DEBUG: Looking for templates at: %s\n", jsonPath)
	data, err := fs.ReadFile(templateFS, jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates.json: %w", err)
	}

	var templatesFile File
	if err := json.Unmarshal(data, &templatesFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal templates.json: %w", err)
	}

	return templatesFile.Templates, nil
}

// GetTemplateByID retrieves a specific template by its ID
func (s *Service) GetTemplateByID(id string) (*Template, error) {
	templates, err := s.GetAvailableTemplates()
	if err != nil {
		return nil, err
	}

	for _, template := range templates {
		if template.ID == id {
			return &template, nil
		}
	}

	return nil, fmt.Errorf("template with id %s not found", id)
}

// GetTemplateContent reads the docker-compose.yml content for a template
func (s *Service) GetTemplateContent(template *Template) (string, error) {
	templateFS, err := embeds.TemplateFS()
	if err != nil {
		return "", fmt.Errorf("failed to get template filesystem: %w", err)
	}

	yamlPath := filepath.Join(s.templatesPath, template.Filename)
	content, err := fs.ReadFile(templateFS, yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", template.Filename, err)
	}

	return string(content), nil
}

// ProcessTemplateContent replaces template variables with actual values
func (s *Service) ProcessTemplateContent(content string, appName string) string {
	// Note: Version locking is now handled by the agent during initial setup
	// This keeps the UI responsive and non-blocking

	// For multi-service apps, we don't modify service names
	// Service names in templates should be descriptive (e.g., "web", "db", "redis")
	// rather than matching the app name

	// Replace platform-specific placeholders
	isDemo := os.Getenv("TREEOS_RUN_MODE") == "demo"

	// {{APP_VOLUMES_PATH}} - Context-aware path to app volumes directory
	// In demo: ./volumes (relative to docker-compose.yml location)
	// In production: OS-specific absolute path
	appVolumesPath := "./volumes"
	if !isDemo {
		appVolumesPath = config.GetAppVolumesPath(appName)
	}
	content = strings.ReplaceAll(content, "{{APP_VOLUMES_PATH}}", appVolumesPath)

	// {{APP_MNT_PATH}} - Context-aware path to app mnt directory
	// In demo: ./mnt (relative to docker-compose.yml location)
	// In production: OS-specific absolute path
	appMntPath := "./mnt"
	if !isDemo {
		appMntPath = config.GetAppMntPath(appName)
	}
	content = strings.ReplaceAll(content, "{{APP_MNT_PATH}}", appMntPath)

	// {{SHARED_OLLAMA_PATH}} - Context-aware path to shared Ollama models
	// In demo: ../../shared/ollama (relative, go up 2 levels from app dir)
	// In production: OS-specific absolute path
	sharedOllamaPath := "../../shared/ollama"
	if !isDemo {
		sharedOllamaPath = config.GetSharedOllamaPath()
	}
	content = strings.ReplaceAll(content, "{{SHARED_OLLAMA_PATH}}", sharedOllamaPath)

	// {{SHARED_PATH}} - Context-aware path to shared directory
	// In demo: ../../shared (relative, go up 2 levels from app dir)
	// In production: OS-specific absolute path
	sharedPath := "../../shared"
	if !isDemo {
		sharedPath = config.GetSharedPath()
	}
	content = strings.ReplaceAll(content, "{{SHARED_PATH}}", sharedPath)

	// {{APP_NAME}} - The name of the app being created
	content = strings.ReplaceAll(content, "{{APP_NAME}}", appName)

	// TODO: In the future, this could support more variable substitution like:
	// {{.Port}}, {{.RandomString}}, etc.

	return content
}

// Note: The following functions are kept for potential future use by the agent
// They are not used during template processing to keep the UI responsive

// lockDockerImageVersions would update images to specific version tags (moved to agent)
// getLatestImageDigest would fetch image digests (moved to agent)
