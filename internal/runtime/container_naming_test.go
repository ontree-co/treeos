package runtime

import (
	"strings"
	"testing"
)

// TestContainerPrefixMatching tests that container name prefix matching works correctly
func TestContainerPrefixMatching(t *testing.T) {
	tests := []struct {
		name          string
		appName       string
		containerName string
		shouldMatch   bool
	}{
		{
			name:          "lowercase app name with matching container",
			appName:       "uptime-kuma",
			containerName: "/ontree-uptime-kuma-uptime-kuma-1",
			shouldMatch:   true,
		},
		{
			name:          "mixed case app name should match lowercase container",
			appName:       "OpenWebUI-0902",
			containerName: "/ontree-openwebui-0902-openwebui-1",
			shouldMatch:   true,
		},
		{
			name:          "container without ontree prefix should not match",
			appName:       "openwebui-0902",
			containerName: "/openwebui-0902-openwebui-1",
			shouldMatch:   false,
		},
		{
			name:          "different app container should not match",
			appName:       "uptime-kuma",
			containerName: "/ontree-openwebui-0902-openwebui-1",
			shouldMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appIdentifier := strings.ToLower(tt.appName)
			prefix := "ontree-" + appIdentifier + "-"
			cleanName := strings.TrimPrefix(tt.containerName, "/")
			matches := strings.HasPrefix(cleanName, prefix)

			if matches != tt.shouldMatch {
				t.Errorf("Container %q matching app %q: got %v, want %v",
					tt.containerName, tt.appName, matches, tt.shouldMatch)
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
