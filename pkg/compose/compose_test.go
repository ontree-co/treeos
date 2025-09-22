package compose

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

func TestComposeIntegration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	// Create compose service
	service, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create compose service: %v", err)
	}

	// Get absolute path to testdata directory
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get testdata directory: %v", err)
	}

	// Verify docker-compose.yml exists
	composeFile := filepath.Join(testdataDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		t.Fatalf("docker-compose.yml not found at %s", composeFile)
	}

	// Set up test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	opts := Options{
		WorkingDir: testdataDir,
	}

	// Test Up operation
	t.Run("Up", func(t *testing.T) {
		err := service.Up(ctx, opts)
		if err != nil {
			t.Fatalf("Failed to start compose stack: %v", err)
		}
		t.Log("Successfully started compose stack")

		// Give services a moment to fully start
		time.Sleep(5 * time.Second)
	})

	// Test PS operation
	t.Run("PS", func(t *testing.T) {
		containers, err := service.PS(ctx, opts)
		if err != nil {
			t.Fatalf("Failed to list containers: %v", err)
		}

		if len(containers) == 0 {
			t.Fatal("Expected at least one container")
		}

		t.Logf("Found %d containers", len(containers))
		for _, container := range containers {
			t.Logf("Container: %s, State: %s, Image: %s", container.Name, container.State, container.Image)
		}
	})

	// Test Down operation (without removing volumes)
	t.Run("Down", func(t *testing.T) {
		err := service.Down(ctx, opts, false)
		if err != nil {
			t.Fatalf("Failed to stop compose stack: %v", err)
		}
		t.Log("Successfully stopped compose stack")
	})

	// Test Down operation with volume removal
	t.Run("DownWithVolumes", func(t *testing.T) {
		// First bring it up again
		err := service.Up(ctx, opts)
		if err != nil {
			t.Fatalf("Failed to start compose stack: %v", err)
		}

		// Give services a moment to start
		time.Sleep(3 * time.Second)

		// Now bring it down with volumes
		err = service.Down(ctx, opts, true)
		if err != nil {
			t.Fatalf("Failed to stop compose stack with volumes: %v", err)
		}
		t.Log("Successfully stopped compose stack and removed volumes")
	})
}

func TestComposeServiceCreation(t *testing.T) {
	// This test doesn't require Docker
	service, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create compose service: %v", err)
	}

	if service == nil {
		t.Fatal("Expected non-nil service")
		return // Explicit return to help the linter understand
	}

	if service.service == nil {
		t.Fatal("Expected non-nil internal service")
	}
}

// TestComposeProjectNaming verifies that the compose Options
// correctly format the project name according to the naming convention
func TestComposeProjectNaming(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		expectedProject string
	}{
		{
			name:            "Simple app name",
			appName:         "myapp",
			expectedProject: "ontree-myapp",
		},
		{
			name:            "App name with hyphens",
			appName:         "my-web-app",
			expectedProject: "ontree-my-web-app",
		},
		{
			name:            "App name with numbers",
			appName:         "app123",
			expectedProject: "ontree-app123",
		},
		{
			name:            "Complex app name",
			appName:         "openwebui-multi",
			expectedProject: "ontree-openwebui-multi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// With the simplified approach, Docker Compose derives project name from directory
			// These tests are no longer relevant as we don't set project names explicitly
			t.Skip("Project names are now derived from directory names by Docker Compose")
		})
	}
}

// TestComposeNamingConventionIntegration verifies that containers created
// by docker-compose follow the expected naming convention
func TestComposeNamingConventionIntegration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	// Create compose service
	service, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create compose service: %v", err)
	}

	// Get absolute path to testdata directory
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get testdata directory: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Test with a specific project name following the convention
	appName := "testapp"
	opts := Options{
		WorkingDir: testdataDir,
	}

	// Bring up the stack
	err = service.Up(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to start compose stack: %v", err)
	}

	// Clean up after test
	defer func() {
		service.Down(context.Background(), opts, true)
	}()

	// Give services a moment to start
	time.Sleep(3 * time.Second)

	// Get container list
	containers, err := service.PS(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	// Verify container naming convention
	for _, container := range containers {
		// Container names should follow: ontree-{appName}-{serviceName}-{index}
		expectedPrefix := "ontree-" + appName + "-"
		if !startsWith(container.Name, expectedPrefix) {
			t.Errorf("Container name '%s' doesn't follow naming convention, expected to start with '%s'",
				container.Name, expectedPrefix)
		}

		t.Logf("Container follows naming convention: %s", container.Name)
	}
}

// Helper function to check string prefix (handles potential leading slash)
func startsWith(s, prefix string) bool {
	// Remove leading slash if present
	if len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestComposeErrorHandling tests the retry logic for various error scenarios
func TestComposeErrorHandling(t *testing.T) {
	// This test validates the error handling logic without requiring Docker

	tests := []struct {
		name          string
		errorMsg      string
		shouldRetry   bool
		retryStrategy string
	}{
		{
			name:          "Container name conflict",
			errorMsg:      `Error response from daemon: Conflict. The container name "/test1-nginx-1" is already in use`,
			shouldRetry:   true,
			retryStrategy: "remove and recreate",
		},
		{
			name:          "No container found",
			errorMsg:      `no container found for project "test1": not found`,
			shouldRetry:   true,
			retryStrategy: "force recreate",
		},
		{
			name:          "Generic error",
			errorMsg:      "network error: connection refused",
			shouldRetry:   false,
			retryStrategy: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test if error would trigger retry logic
			hasConflict := contains(tt.errorMsg, "Conflict. The container name") &&
				contains(tt.errorMsg, "is already in use")
			hasNoContainer := contains(tt.errorMsg, "no container found")

			shouldRetry := hasConflict || hasNoContainer

			if shouldRetry != tt.shouldRetry {
				t.Errorf("Expected retry=%v for error: %s", tt.shouldRetry, tt.errorMsg)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}

// TestContainerLabelsIntegration tests that containers are created with proper Docker Compose labels
func TestContainerLabelsIntegration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	// Create compose service
	service, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create compose service: %v", err)
	}
	defer service.Close()

	// Create a temporary directory for test
	tmpDir := t.TempDir()

	// Create a simple docker-compose.yml
	composeContent := `version: '3.8'
services:
  web:
    image: alpine:latest
    command: ["sleep", "300"]
`
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	// Test with specific project name
	projectName := "labeltest"
	opts := Options{
		WorkingDir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up any existing containers
	_ = service.Down(ctx, opts, true)

	// Start the containers
	err = service.Up(ctx, opts)
	if err != nil {
		// Check if it's the known "no container found" error
		if !strings.Contains(err.Error(), "no container found") {
			t.Fatalf("Failed to start compose stack: %v", err)
		}
		t.Logf("Got expected error: %v", err)

		// For this test, we want to check the labels even if start failed
		// because the issue is that containers are created without labels
	}

	// Clean up after test
	defer func() {
		downCtx, downCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer downCancel()
		_ = service.Down(downCtx, opts, true)
	}()

	// Give container a moment to start
	time.Sleep(2 * time.Second)

	// List all containers (including non-running) by name pattern
	// Since PS might not return containers without labels, we'll use docker directly
	containerList, err := service.dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", projectName)),
	})
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	if len(containerList) == 0 {
		t.Fatal("No containers found")
	}

	t.Logf("Found %d containers", len(containerList))

	// Check labels on the found containers
	for _, cont := range containerList {
		// Container names include ID or name
		containerID := cont.ID

		// Log container info
		containerName := "unknown"
		if len(cont.Names) > 0 {
			containerName = strings.TrimPrefix(cont.Names[0], "/")
		}
		t.Logf("Checking container: %s (ID: %s)", containerName, containerID[:12])

		// Check for required labels
		labels := cont.Labels

		// Check com.docker.compose.project label
		if projectLabel, exists := labels["com.docker.compose.project"]; !exists {
			t.Errorf("Container %s missing 'com.docker.compose.project' label", containerName)
		} else if projectLabel != projectName {
			t.Errorf("Container %s has project label '%s', expected '%s'", containerName, projectLabel, projectName)
		}

		// Check com.docker.compose.service label
		if _, exists := labels["com.docker.compose.service"]; !exists {
			t.Errorf("Container %s missing 'com.docker.compose.service' label", containerName)
		}

		// Log all labels for debugging
		t.Logf("Container %s labels:", containerName)
		for k, v := range labels {
			if strings.HasPrefix(k, "com.docker.compose") {
				t.Logf("  %s: %s", k, v)
			}
		}
	}
}
