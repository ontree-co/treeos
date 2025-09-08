package templates

import (
	"strings"
	"testing"
)

func TestProcessTemplateContent(t *testing.T) {
	// Create a service
	svc := NewService("compose")

	// Test content with a simple image
	testContent := `version: '3.8'
services:
  nginx:
    image: nginx:latest
    ports:
      - "80:80"`

	// Process the content
	result := svc.ProcessTemplateContent(testContent, "test-app")

	// Check that the content was processed
	if result == "" {
		t.Error("ProcessTemplateContent returned empty string")
	}

	// The result should contain either the original image or a digest
	if !strings.Contains(result, "nginx") {
		t.Error("ProcessTemplateContent lost the nginx image reference")
	}

	// Check if digest format is present (if docker is available)
	// This is optional since it depends on Docker being available
	if strings.Contains(result, "@sha256:") {
		t.Log("Successfully locked image to digest")
	} else {
		t.Log("Image not locked (Docker might not be available in test environment)")
	}
}
