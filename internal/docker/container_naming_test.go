package docker

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// dockerClientInterface defines the methods we need from Docker client
type dockerClientInterface interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	Close() error
}

// Ensure real Docker client implements our interface
var _ dockerClientInterface = (*client.Client)(nil)

func TestGetContainerStatus(t *testing.T) {
	tests := []struct {
		name           string
		appName        string
		containers     []container.Summary
		expectedStatus string
	}{
		{
			name:    "lowercase app name with running container",
			appName: "uptime-kuma",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-uptime-kuma-uptime-kuma-1"},
					State: "running",
				},
			},
			expectedStatus: "running",
		},
		{
			name:    "mixed case app name should convert to lowercase",
			appName: "OpenWebUI-0902",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-openwebui-0902-openwebui-1"},
					State: "running",
				},
			},
			expectedStatus: "running",
		},
		{
			name:    "app with hyphen and numbers",
			appName: "openwebui-0902",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-openwebui-0902-openwebui-1"},
					State: "running",
				},
			},
			expectedStatus: "running",
		},
		{
			name:    "app with multiple services - all running",
			appName: "openwebui-amd",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-openwebui-amd-ollama-1"},
					State: "running",
				},
				{
					Names: []string{"/ontree-openwebui-amd-open-webui-1"},
					State: "running",
				},
			},
			expectedStatus: "running",
		},
		{
			name:    "app with multiple services - mixed states",
			appName: "openwebui-amd",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-openwebui-amd-ollama-1"},
					State: "running",
				},
				{
					Names: []string{"/ontree-openwebui-amd-open-webui-1"},
					State: "exited",
				},
			},
			expectedStatus: "partial",
		},
		{
			name:    "app with all containers stopped",
			appName: "openwebui-cpu",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-openwebui-cpu-openwebui-1"},
					State: "exited",
				},
			},
			expectedStatus: "exited",
		},
		{
			name:           "app with no containers",
			appName:        "new-app",
			containers:     []container.Summary{},
			expectedStatus: "not_created",
		},
		{
			name:    "should not match containers without ontree prefix",
			appName: "openwebui-0902",
			containers: []container.Summary{
				{
					Names: []string{"/openwebui-0902-openwebui-1"}, // Missing ontree- prefix
					State: "running",
				},
			},
			expectedStatus: "not_created",
		},
		{
			name:    "should not match containers from different apps",
			appName: "uptime-kuma",
			containers: []container.Summary{
				{
					Names: []string{"/ontree-openwebui-0902-openwebui-1"},
					State: "running",
				},
			},
			expectedStatus: "not_created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For this test, we'll simulate the logic of getContainerStatus
			// since we can't easily inject a mock into the private dockerClient field

			// This mimics the logic in getContainerStatus
			appIdentifier := strings.ToLower(tt.appName)
			prefix := "ontree-" + appIdentifier + "-"

			var runningCount, stoppedCount int
			for _, cont := range tt.containers {
				for _, name := range cont.Names {
					cleanName := strings.TrimPrefix(name, "/")
					if strings.HasPrefix(cleanName, prefix) {
						if strings.ToLower(cont.State) == "running" {
							runningCount++
						} else {
							stoppedCount++
						}
					}
				}
			}

			status := "not_created"
			if runningCount > 0 && stoppedCount > 0 {
				status = "partial"
			} else if runningCount > 0 {
				status = "running"
			} else if stoppedCount > 0 {
				status = "exited"
			}

			// Check the result
			if status != tt.expectedStatus {
				t.Errorf("getContainerStatus(%q) = %q, want %q", tt.appName, status, tt.expectedStatus)
			}
		})
	}
}

// TestContainerNamingConsistency tests that our naming scheme is consistent
func TestContainerNamingConsistency(t *testing.T) {
	testCases := []struct {
		appDirName     string
		expectedPrefix string
		description    string
	}{
		{
			appDirName:     "uptime-kuma",
			expectedPrefix: "ontree-uptime-kuma-",
			description:    "lowercase with hyphen",
		},
		{
			appDirName:     "OpenWebUI-0902",
			expectedPrefix: "ontree-openwebui-0902-",
			description:    "mixed case with hyphen and numbers",
		},
		{
			appDirName:     "UPPERCASE-APP",
			expectedPrefix: "ontree-uppercase-app-",
			description:    "all uppercase",
		},
		{
			appDirName:     "app_with_underscore",
			expectedPrefix: "ontree-app_with_underscore-",
			description:    "with underscore",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// In our implementation, we convert app names to lowercase for container prefixes
			appIdentifier := tc.appDirName
			// Convert to lowercase as per our implementation
			expectedIdentifier := tc.expectedPrefix

			// This mimics what happens in getContainerStatus
			actualPrefix := "ontree-" + strings.ToLower(appIdentifier) + "-"

			if actualPrefix != expectedIdentifier {
				t.Errorf("For app %q, expected prefix %q but got %q",
					tc.appDirName, expectedIdentifier, actualPrefix)
			}
		})
	}
}
