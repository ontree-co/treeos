// Package templates provides template management functionality for application templates
package templates

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ontree-co/treeos/internal/config"
	"github.com/ontree-co/treeos/internal/embeds"
)

// Template represents an application template
type Template struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Category         string   `json:"category,omitempty"`      // legacy single category support
	CategoryTags     []string `json:"category_tags,omitempty"` // preferred multi-category tags
	Icon             string   `json:"icon"`
	Filename         string   `json:"filename"`
	Port             string   `json:"port"`
	DocumentationURL string   `json:"documentation_url"`
	IsSystemService  bool     `json:"is_system_service,omitempty"`
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
	templateFS, err := embeds.AppTemplateFS()
	if err != nil {
		return nil, fmt.Errorf("failed to get template filesystem: %w", err)
	}

	entries, err := fs.ReadDir(templateFS, s.templatesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates directory: %w", err)
	}

	templates := make([]Template, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		jsonPath := filepath.Join(s.templatesPath, dirName, "template.json")
		fmt.Printf("DEBUG: Looking for template metadata at: %s\n", jsonPath)

		data, err := fs.ReadFile(templateFS, jsonPath)
		if err != nil {
			// Skip directories without template.json
			fmt.Printf("DEBUG: Skipping %s (no template.json)\n", dirName)
			continue
		}

		var tmpl Template
		if err := json.Unmarshal(data, &tmpl); err != nil {
			fmt.Printf("DEBUG: Failed to unmarshal %s: %v\n", jsonPath, err)
			continue
		}

		// Default sensible values
		if tmpl.ID == "" {
			tmpl.ID = dirName
		}
		if tmpl.Filename == "" {
			tmpl.Filename = "docker-compose.yml"
		}
		if len(tmpl.CategoryTags) == 0 && tmpl.Category != "" {
			tmpl.CategoryTags = []string{tmpl.Category}
		}

		templates = append(templates, tmpl)
	}

	return templates, nil
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
	templateFS, err := embeds.AppTemplateFS()
	if err != nil {
		return "", fmt.Errorf("failed to get template filesystem: %w", err)
	}

	yamlPath := filepath.Join(s.templatesPath, template.ID, template.Filename)
	content, err := fs.ReadFile(templateFS, yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", template.Filename, err)
	}

	return string(content), nil
}

// GetTemplateEnvExample reads the .env.example file for a template if it exists
// Returns empty string (not an error) if the .env.example file doesn't exist
func (s *Service) GetTemplateEnvExample(templateID string) (string, error) {
	templateFS, err := embeds.AppTemplateFS()
	if err != nil {
		return "", fmt.Errorf("failed to get template filesystem: %w", err)
	}

	// .env.example lives inside the template directory
	envExamplePath := filepath.Join(s.templatesPath, templateID, ".env.example")

	// Try to read the file - if it doesn't exist, return empty string (not an error)
	content, err := fs.ReadFile(templateFS, envExamplePath)
	if err != nil {
		// File doesn't exist - this is normal for templates without .env.example
		return "", nil
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
