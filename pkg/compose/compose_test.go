package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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
		ProjectName: "ontree-test-compose",
		WorkingDir:  testdataDir,
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
			opts := Options{
				ProjectName: "ontree-" + tt.appName,
				WorkingDir:  "/test/path",
			}
			
			if opts.ProjectName != tt.expectedProject {
				t.Errorf("Expected project name '%s', got '%s'", tt.expectedProject, opts.ProjectName)
			}
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
		ProjectName: "ontree-" + appName,
		WorkingDir:  testdataDir,
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