package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GenerateAppConfigFromCompose generates an app.yml file from docker-compose.yml
func GenerateAppConfigFromCompose(appDir string) error {
	composePath := filepath.Join(appDir, "docker-compose.yml")

	// Read docker-compose.yml
	composeData, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("failed to read docker-compose.yml: %w", err)
	}

	// Parse docker-compose.yml
	var compose map[string]interface{}
	if err := yaml.Unmarshal(composeData, &compose); err != nil {
		return fmt.Errorf("failed to parse docker-compose.yml: %w", err)
	}

	// Extract app name from directory and convert to lowercase
	appName := strings.ToLower(filepath.Base(appDir))

	// Extract services
	services := []string{}
	primaryService := ""

	if servicesMap, ok := compose["services"].(map[string]interface{}); ok {
		for serviceName := range servicesMap {
			// Use the new naming pattern: ontree-<app_identifier>-<service_name>-<instance>
			containerName := fmt.Sprintf("ontree-%s-%s-1", appName, serviceName)
			services = append(services, containerName)
			if primaryService == "" {
				// Use first service as primary by default
				primaryService = containerName
			}
		}
	}

	// Extract x-ontree metadata if available (for future use)
	// var subdomain string
	// if xOntree, ok := compose["x-ontree"].(map[string]interface{}); ok {
	// 	if sd, ok := xOntree["subdomain"].(string); ok {
	// 		subdomain = sd
	// 	}
	// }

	// Create app config - use just app name as ID (no prefix)
	appConfig := AppConfig{
		ID:               appName,
		Name:             appName,
		PrimaryService:   primaryService,
		ExpectedServices: services,
	}

	// Add Uptime Kuma monitor if it's the uptime-kuma app itself
	if strings.Contains(appName, "uptime") {
		appConfig.UptimeKumaMonitor = ""
	}

	// Write app.yml
	appYmlPath := filepath.Join(appDir, "app.yml")
	appYmlData, err := yaml.Marshal(appConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal app config: %w", err)
	}

	if err := os.WriteFile(appYmlPath, appYmlData, 0600); err != nil {
		return fmt.Errorf("failed to write app.yml: %w", err)
	}

	return nil
}

// GenerateAllAppConfigs generates app.yml files for all apps in the directory
func GenerateAllAppConfigs(appsDir string) error {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("failed to read apps directory: %w", err)
	}

	generated := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appDir := filepath.Join(appsDir, entry.Name())
		composePath := filepath.Join(appDir, "docker-compose.yml")
		appYmlPath := filepath.Join(appDir, "app.yml")

		// Check if docker-compose.yml exists
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			continue
		}

		// Skip if app.yml already exists
		if _, err := os.Stat(appYmlPath); err == nil {
			fmt.Printf("Skipping %s: app.yml already exists\n", entry.Name())
			continue
		}

		// Generate app.yml
		if err := GenerateAppConfigFromCompose(appDir); err != nil {
			fmt.Printf("Failed to generate app.yml for %s: %v\n", entry.Name(), err)
			continue
		}

		fmt.Printf("Generated app.yml for %s\n", entry.Name())
		generated++
	}

	fmt.Printf("Generated %d app.yml files\n", generated)
	return nil
}
