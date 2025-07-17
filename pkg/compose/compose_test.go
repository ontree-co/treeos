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