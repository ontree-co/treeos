// Package templates provides template management functionality for Docker application templates
package templates

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"treeos/internal/config"
	"treeos/internal/embeds"
)

// Template represents a Docker application template
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

// GetAvailableTemplates returns all available Docker application templates
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
	// {{SHARED_MODELS_PATH}} - Path to shared models directory (platform-specific)
	sharedModelsPath := config.GetSharedModelsPath()
	content = strings.ReplaceAll(content, "{{SHARED_MODELS_PATH}}", sharedModelsPath)

	// TODO: In the future, this could support more variable substitution like:
	// {{.Port}}, {{.AppName}}, {{.RandomString}}, etc.

	return content
}

// Note: The following functions are kept for potential future use by the agent
// They are not used during template processing to keep the UI responsive

// lockDockerImageVersions would update images to specific version tags (moved to agent)
// getLatestImageDigest would fetch image digests (moved to agent)
