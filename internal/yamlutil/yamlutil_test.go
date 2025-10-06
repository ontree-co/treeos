package yamlutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteComposeWithMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "yamlutil-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

	// Test case 1: Basic compose file with comments
	testFile := filepath.Join(tempDir, "docker-compose.yml")
	originalContent := `# This is a test compose file
version: '3.8'

# Services section
services:
  myapp:
    image: nginx:latest  # Web server
    ports:
      - "8080:80"
    environment:
      - ENV=production
`

	// Write the original content
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read the compose file
	compose, err := ReadComposeWithMetadata(testFile)
	if err != nil {
		t.Fatalf("Failed to read compose file: %v", err)
	}

	// Verify basic structure
	if compose.Version != "3.8" {
		t.Errorf("Expected version 3.8, got %s", compose.Version)
	}

	// Add OnTree metadata
	metadata := &OnTreeMetadata{
		Subdomain: "myapp",
		HostPort:  8080,
		IsExposed: true,
	}
	SetOnTreeMetadata(compose, metadata)

	// Write back the file
	if err := WriteComposeWithMetadata(testFile, compose); err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	// Read the file again to verify
	compose2, err := ReadComposeWithMetadata(testFile)
	if err != nil {
		t.Fatalf("Failed to read modified compose file: %v", err)
	}

	// Verify metadata was preserved
	if compose2.XOnTree == nil {
		t.Fatal("OnTree metadata is nil")
	}
	if compose2.XOnTree.Subdomain != "myapp" {
		t.Errorf("Expected subdomain 'myapp', got '%s'", compose2.XOnTree.Subdomain)
	}
	if compose2.XOnTree.HostPort != 8080 {
		t.Errorf("Expected host_port 8080, got %d", compose2.XOnTree.HostPort)
	}
	if !compose2.XOnTree.IsExposed {
		t.Error("Expected is_exposed to be true")
	}

	// Read the raw content to check if comments were preserved
	content, err := os.ReadFile(testFile) //nolint:gosec // Test file read
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}

	// Check that the file contains x-ontree section
	if !strings.Contains(string(content), "x-ontree:") {
		t.Error("File doesn't contain x-ontree section")
	}
}

func TestReadComposeMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "yamlutil-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

	// Create a compose file with metadata
	testContent := `version: '3.8'
x-ontree:
  subdomain: testapp
  host_port: 3000
  is_exposed: false
services:
  app:
    image: node:14
`

	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(testContent), 0644); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test ReadComposeMetadata helper
	metadata, err := ReadComposeMetadata(tempDir)
	if err != nil {
		t.Fatalf("Failed to read compose metadata: %v", err)
	}

	if metadata.Subdomain != "testapp" {
		t.Errorf("Expected subdomain 'testapp', got '%s'", metadata.Subdomain)
	}
	if metadata.HostPort != 3000 {
		t.Errorf("Expected host_port 3000, got %d", metadata.HostPort)
	}
	if metadata.IsExposed {
		t.Error("Expected is_exposed to be false")
	}
}

func TestUpdateComposeMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "yamlutil-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

	// Create a simple compose file
	testContent := `version: '3'
services:
  web:
    image: nginx
`

	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(testContent), 0644); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Update metadata
	newMetadata := &OnTreeMetadata{
		Subdomain: "webapp",
		HostPort:  80,
		IsExposed: true,
	}

	if err := UpdateComposeMetadata(tempDir, newMetadata); err != nil {
		t.Fatalf("Failed to update compose metadata: %v", err)
	}

	// Verify the update
	metadata, err := ReadComposeMetadata(tempDir)
	if err != nil {
		t.Fatalf("Failed to read updated metadata: %v", err)
	}

	if metadata.Subdomain != "webapp" {
		t.Errorf("Expected subdomain 'webapp', got '%s'", metadata.Subdomain)
	}
	if metadata.HostPort != 80 {
		t.Errorf("Expected host_port 80, got %d", metadata.HostPort)
	}
	if !metadata.IsExposed {
		t.Error("Expected is_exposed to be true")
	}
}

func TestEmptyMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "yamlutil-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

	// Create a compose file without metadata
	testContent := `version: '3'
services:
  db:
    image: postgres
`

	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(testContent), 0644); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read metadata - should return empty struct
	metadata, err := ReadComposeMetadata(tempDir)
	if err != nil {
		t.Fatalf("Failed to read compose metadata: %v", err)
	}

	if metadata.Subdomain != "" {
		t.Errorf("Expected empty subdomain, got '%s'", metadata.Subdomain)
	}
	if metadata.HostPort != 0 {
		t.Errorf("Expected host_port 0, got %d", metadata.HostPort)
	}
	if metadata.IsExposed {
		t.Error("Expected is_exposed to be false")
	}
}

func TestEmojiFunctionality(t *testing.T) {
	// Test case 1: Valid emoji validation
	t.Run("ValidEmoji", func(t *testing.T) {
		validEmojis := []string{"🚀", "💻", "🔒", "📊", "🌍", ""}
		for _, emoji := range validEmojis {
			if !IsValidEmoji(emoji) {
				t.Errorf("Expected emoji '%s' to be valid", emoji)
			}
		}
	})

	// Test case 2: Invalid emoji validation
	t.Run("InvalidEmoji", func(t *testing.T) {
		invalidEmojis := []string{"😀", "🎈", "🍕", "invalid", "123"}
		for _, emoji := range invalidEmojis {
			if IsValidEmoji(emoji) {
				t.Errorf("Expected emoji '%s' to be invalid", emoji)
			}
		}
	})

	// Test case 3: Emoji storage in YAML
	t.Run("EmojiStorageInYAML", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "yamlutil-emoji-test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

		// Create a compose file
		testContent := `version: '3.8'
services:
  app:
    image: nginx:latest
`
		composeFile := filepath.Join(tempDir, "docker-compose.yml")
		if err := os.WriteFile(composeFile, []byte(testContent), 0644); err != nil { //nolint:gosec // Test file permissions
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Update metadata with emoji
		metadata := &OnTreeMetadata{
			Subdomain: "myapp",
			HostPort:  8080,
			IsExposed: true,
			Emoji:     "🚀",
		}

		if err := UpdateComposeMetadata(tempDir, metadata); err != nil {
			t.Fatalf("Failed to update compose metadata: %v", err)
		}

		// Read back and verify
		readMetadata, err := ReadComposeMetadata(tempDir)
		if err != nil {
			t.Fatalf("Failed to read compose metadata: %v", err)
		}

		if readMetadata.Emoji != "🚀" {
			t.Errorf("Expected emoji '🚀', got '%s'", readMetadata.Emoji)
		}
	})

	// Test case 4: Emoji persistence through multiple updates
	t.Run("EmojiPersistence", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "yamlutil-emoji-persist")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

		// Create initial compose file with emoji
		testContent := `version: '3.8'
x-ontree:
  subdomain: testapp
  host_port: 3000
  is_exposed: false
  emoji: "💻"
services:
  app:
    image: node:14
`
		composeFile := filepath.Join(tempDir, "docker-compose.yml")
		if err := os.WriteFile(composeFile, []byte(testContent), 0644); err != nil { //nolint:gosec // Test file permissions
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Read metadata
		metadata, err := ReadComposeMetadata(tempDir)
		if err != nil {
			t.Fatalf("Failed to read compose metadata: %v", err)
		}

		if metadata.Emoji != "💻" {
			t.Errorf("Expected emoji '💻', got '%s'", metadata.Emoji)
		}

		// Update other fields without changing emoji
		metadata.Subdomain = "newsubdomain"
		metadata.IsExposed = true

		if err := UpdateComposeMetadata(tempDir, metadata); err != nil {
			t.Fatalf("Failed to update compose metadata: %v", err)
		}

		// Read again to verify emoji was preserved
		updatedMetadata, err := ReadComposeMetadata(tempDir)
		if err != nil {
			t.Fatalf("Failed to read updated metadata: %v", err)
		}

		if updatedMetadata.Emoji != "💻" {
			t.Errorf("Expected emoji '💻' to be preserved, got '%s'", updatedMetadata.Emoji)
		}
		if updatedMetadata.Subdomain != "newsubdomain" {
			t.Errorf("Expected subdomain 'newsubdomain', got '%s'", updatedMetadata.Subdomain)
		}
	})

	// Test case 5: Empty emoji handling
	t.Run("EmptyEmoji", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "yamlutil-emoji-empty")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir) //nolint:errcheck // Test cleanup

		// Create compose file without emoji
		testContent := `version: '3.8'
services:
  app:
    image: nginx:latest
`
		composeFile := filepath.Join(tempDir, "docker-compose.yml")
		if err := os.WriteFile(composeFile, []byte(testContent), 0644); err != nil { //nolint:gosec // Test file permissions
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Read metadata - emoji should be empty
		metadata, err := ReadComposeMetadata(tempDir)
		if err != nil {
			t.Fatalf("Failed to read compose metadata: %v", err)
		}

		if metadata.Emoji != "" {
			t.Errorf("Expected empty emoji, got '%s'", metadata.Emoji)
		}
	})
}
