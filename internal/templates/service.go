package templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Template struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Icon             string `json:"icon"`
	Filename         string `json:"filename"`
	Port             string `json:"port"`
	DocumentationURL string `json:"documentation_url"`
}

type TemplatesFile struct {
	Templates []Template `json:"templates"`
}

type Service struct {
	templatesPath string
}

func NewService(templatesPath string) *Service {
	return &Service{
		templatesPath: templatesPath,
	}
}

func (s *Service) GetAvailableTemplates() ([]Template, error) {
	jsonPath := filepath.Join(s.templatesPath, "templates.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates.json: %w", err)
	}

	var templatesFile TemplatesFile
	if err := json.Unmarshal(data, &templatesFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal templates.json: %w", err)
	}

	return templatesFile.Templates, nil
}

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

func (s *Service) GetTemplateContent(template *Template) (string, error) {
	yamlPath := filepath.Join(s.templatesPath, template.Filename)
	content, err := os.ReadFile(yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", template.Filename, err)
	}

	return string(content), nil
}

func (s *Service) ProcessTemplateContent(content string, appName string) string {
	// Replace the service name in the compose file with the app name
	lines := strings.Split(content, "\n")

	// For now, we'll do a simple replacement of the first service name
	// In a real implementation, we'd parse the YAML properly
	for i, line := range lines {
		if i == 1 && strings.HasPrefix(line, "  ") && strings.HasSuffix(line, ":") {
			// This is likely the service name
			lines[i] = "  " + appName + ":"
			break
		}
	}

	return strings.Join(lines, "\n")
}
