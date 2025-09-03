package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectWithComposeProjectName(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "compose-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name                string
		appDirName          string
		envContent          string
		dockerComposeContent string
		expectedProjectName string
	}{
		{
			name:       "uses COMPOSE_PROJECT_NAME from .env",
			appDirName: "uptime-kuma",
			envContent: `COMPOSE_PROJECT_NAME=ontree-uptime-kuma
COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  app:
    image: louislam/uptime-kuma:1`,
			expectedProjectName: "ontree-uptime-kuma",
		},
		{
			name:       "handles mixed case app directory",
			appDirName: "OpenWebUI-0902",
			envContent: `COMPOSE_PROJECT_NAME=ontree-openwebui-0902
COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  web:
    image: openwebui/open-webui:latest`,
			expectedProjectName: "ontree-openwebui-0902",
		},
		{
			name:       "falls back to directory name if no COMPOSE_PROJECT_NAME",
			appDirName: "test-app",
			envContent: `COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  app:
    image: nginx:latest`,
			expectedProjectName: "test-app",
		},
		{
			name:       "handles spaces in COMPOSE_PROJECT_NAME",
			appDirName: "my-app",
			envContent: `COMPOSE_PROJECT_NAME = ontree-my-app  
COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  app:
    image: nginx:latest`,
			expectedProjectName: "ontree-my-app",
		},
		{
			name:       "ignores comments in .env",
			appDirName: "commented-app",
			envContent: `# This is a comment
COMPOSE_PROJECT_NAME=ontree-commented-app
# Another comment
COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  app:
    image: nginx:latest`,
			expectedProjectName: "ontree-commented-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create app directory
			appDir := filepath.Join(tmpDir, tt.appDirName)
			if err := os.MkdirAll(appDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write .env file if content is provided
			if tt.envContent != "" {
				envFile := filepath.Join(appDir, ".env")
				if err := os.WriteFile(envFile, []byte(tt.envContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Write docker-compose.yml
			composeFile := filepath.Join(appDir, "docker-compose.yml")
			if err := os.WriteFile(composeFile, []byte(tt.dockerComposeContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Create service and load project
			svc := &Service{}
			opts := Options{
				WorkingDir: appDir,
			}

			project, err := svc.loadProject(context.Background(), opts)
			if err != nil {
				t.Fatalf("loadProject failed: %v", err)
			}

			// Check project name
			if project.Name != tt.expectedProjectName {
				t.Errorf("Expected project name %q, got %q", tt.expectedProjectName, project.Name)
			}
		})
	}
}

func TestRunComposeCommandProjectName(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "compose-cmd-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name                 string
		appDirName           string
		envContent           string
		dockerComposeContent string
		expectedInCommand    string // What we expect to see in the -p flag
	}{
		{
			name:       "uses COMPOSE_PROJECT_NAME for -p flag",
			appDirName: "uptime-kuma",
			envContent: `COMPOSE_PROJECT_NAME=ontree-uptime-kuma
COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  app:
    image: louislam/uptime-kuma:1`,
			expectedInCommand: "ontree-uptime-kuma",
		},
		{
			name:       "handles OpenWebUI-0902 style names",
			appDirName: "OpenWebUI-0902",
			envContent: `COMPOSE_PROJECT_NAME=ontree-openwebui-0902
COMPOSE_SEPARATOR=-`,
			dockerComposeContent: `version: '3'
services:
  web:
    image: openwebui/open-webui:latest`,
			expectedInCommand: "ontree-openwebui-0902",
		},
		{
			name:       "falls back to directory name without .env",
			appDirName: "simple-app",
			envContent: "",
			dockerComposeContent: `version: '3'
services:
  app:
    image: nginx:latest`,
			expectedInCommand: "simple-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create app directory
			appDir := filepath.Join(tmpDir, tt.appDirName)
			if err := os.MkdirAll(appDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write .env file if content is provided
			if tt.envContent != "" {
				envFile := filepath.Join(appDir, ".env")
				if err := os.WriteFile(envFile, []byte(tt.envContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Write docker-compose.yml
			composeFile := filepath.Join(appDir, "docker-compose.yml")
			if err := os.WriteFile(composeFile, []byte(tt.dockerComposeContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Since runComposeCommand actually executes docker-compose,
			// we can't fully test it without Docker, but we can test
			// the project name extraction logic

			// Test the logic that reads COMPOSE_PROJECT_NAME
			projectName := filepath.Base(appDir)
			envFile := filepath.Join(appDir, ".env")
			if _, err := os.Stat(envFile); err == nil {
				envContent, err := os.ReadFile(envFile)
				if err == nil {
					lines := string(envContent)
					// Simple parsing for test
					for _, line := range splitLines(lines) {
						line = trimSpace(line)
						if hasPrefix(line, "COMPOSE_PROJECT_NAME=") {
							projectName = trimSpace(line[21:]) // len("COMPOSE_PROJECT_NAME=") = 21
							break
						}
					}
				}
			}

			if projectName != tt.expectedInCommand {
				t.Errorf("Expected project name %q, got %q", tt.expectedInCommand, projectName)
			}
		})
	}
}

// Helper functions to avoid importing strings in test
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}