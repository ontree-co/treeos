package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanApps(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ontree-test-apps-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test app directories with docker-compose.yml files
	testApps := []struct {
		name    string
		compose string
		emoji   string
	}{
		{
			name: "app1",
			compose: `version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
x-ontree:
  emoji: "🚀"
`,
			emoji: "🚀",
		},
		{
			name: "app2",
			compose: `version: '3.8'
services:
  db:
    image: postgres:13
    environment:
      - POSTGRES_PASSWORD=example
  app:
    image: node:14
    ports:
      - "3000:3000"
`,
			emoji: "",
		},
	}

	// Create test app directories
	for _, testApp := range testApps {
		appDir := filepath.Join(tmpDir, testApp.name)
		if err := os.MkdirAll(appDir, 0755); err != nil {
			t.Fatal(err)
		}

		composeFile := filepath.Join(appDir, "docker-compose.yml")
		if err := os.WriteFile(composeFile, []byte(testApp.compose), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also create a directory without docker-compose.yml (should be ignored)
	noComposeDir := filepath.Join(tmpDir, "no-compose-app")
	if err := os.MkdirAll(noComposeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file (not a directory, should be ignored)
	regularFile := filepath.Join(tmpDir, "regular-file.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Docker client (this will require Docker to be available)
	client, err := NewClient()
	if err != nil {
		t.Skip("Docker not available, skipping test:", err)
	}
	defer client.Close()

	// Test ScanApps
	apps, err := client.ScanApps(tmpDir)
	if err != nil {
		t.Fatalf("ScanApps failed: %v", err)
	}

	// Verify we found the correct number of apps
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
		for _, app := range apps {
			t.Logf("Found app: %s", app.Name)
		}
	}

	// Verify app details
	appMap := make(map[string]*App)
	for _, app := range apps {
		appMap[app.Name] = app
	}

	// Check app1
	if app1, ok := appMap["app1"]; ok {
		if app1.Emoji != "🚀" {
			t.Errorf("app1: expected emoji '🚀', got '%s'", app1.Emoji)
		}
		if len(app1.Services) != 1 {
			t.Errorf("app1: expected 1 service, got %d", len(app1.Services))
		}
		if webService, ok := app1.Services["web"]; ok {
			if webService.Image != "nginx:latest" {
				t.Errorf("app1 web service: expected image 'nginx:latest', got '%s'", webService.Image)
			}
			if len(webService.Ports) != 1 || webService.Ports[0] != "8080:80" {
				t.Errorf("app1 web service: incorrect ports configuration")
			}
		} else {
			t.Error("app1: missing 'web' service")
		}
	} else {
		t.Error("app1 not found in scan results")
	}

	// Check app2
	if app2, ok := appMap["app2"]; ok {
		if app2.Emoji != "" {
			t.Errorf("app2: expected no emoji, got '%s'", app2.Emoji)
		}
		if len(app2.Services) != 2 {
			t.Errorf("app2: expected 2 services, got %d", len(app2.Services))
		}
		if _, ok := app2.Services["db"]; !ok {
			t.Error("app2: missing 'db' service")
		}
		if _, ok := app2.Services["app"]; !ok {
			t.Error("app2: missing 'app' service")
		}
	} else {
		t.Error("app2 not found in scan results")
	}
}

func TestScanApps_RealDirectory(t *testing.T) {
	// Test scanning the actual /opt/ontree/apps directory if it exists
	appsDir := "/opt/ontree/apps"

	// Check if directory exists
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		t.Skip("Apps directory does not exist:", appsDir)
	}

	// Create Docker client
	client, err := NewClient()
	if err != nil {
		t.Skip("Docker not available, skipping test:", err)
	}
	defer client.Close()

	// Scan the real apps directory
	apps, err := client.ScanApps(appsDir)
	if err != nil {
		t.Fatalf("ScanApps failed for real directory: %v", err)
	}

	// Log what we found
	t.Logf("Found %d apps in %s:", len(apps), appsDir)
	for _, app := range apps {
		t.Logf("  - %s (emoji: %s, status: %s, services: %d)",
			app.Name, app.Emoji, app.Status, len(app.Services))
		for serviceName, service := range app.Services {
			t.Logf("    - Service %s: %s", serviceName, service.Image)
		}
		if app.Error != "" {
			t.Logf("    Error: %s", app.Error)
		}
	}

	// The actual apps should be found (update based on what's actually in the directory)
	// We just check that we found some apps, not specific ones since the directory contents can change
	if len(apps) == 0 {
		t.Error("Expected to find some apps, but got none")
	}

	// Check that at least some known apps were found
	// Updated to reflect actual apps in the test directory
	expectedApps := []string{
		"ollama-amd",
		"ollama-cpu",
	}

	// Check that each expected app was found
	foundApps := make(map[string]bool)
	for _, app := range apps {
		foundApps[app.Name] = true
	}

	for _, expectedApp := range expectedApps {
		if !foundApps[expectedApp] {
			t.Errorf("Expected app '%s' not found", expectedApp)
		}
	}
}
