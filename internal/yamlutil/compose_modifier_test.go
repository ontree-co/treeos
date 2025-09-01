package yamlutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAddTailscaleSidecar(t *testing.T) {
	// Create a basic compose file
	compose := &ComposeFile{
		Version: "3.8",
		Services: map[string]interface{}{
			"webapp": map[string]interface{}{
				"image": "nginx:alpine",
				"ports": []interface{}{"8080:80"},
			},
		},
	}

	// Add Tailscale sidecar
	err := AddTailscaleSidecar(compose, "testapp", "testhost", "tskey-auth-test")
	if err != nil {
		t.Fatalf("Failed to add Tailscale sidecar: %v", err)
	}

	// Check that Tailscale service was added
	if _, exists := compose.Services["tailscale"]; !exists {
		t.Error("Tailscale service was not added")
	}

	// Check that main service was modified
	webapp, ok := compose.Services["webapp"].(map[string]interface{})
	if !ok {
		t.Fatal("webapp service not found or wrong type")
	}

	// Check network_mode was set
	if networkMode, exists := webapp["network_mode"]; !exists || networkMode != "service:tailscale" {
		t.Errorf("network_mode not set correctly, got: %v", webapp["network_mode"])
	}

	// Check original ports were saved
	if _, exists := webapp["x-original-ports"]; !exists {
		t.Error("Original ports were not saved")
	}

	// Check ports were removed
	if _, exists := webapp["ports"]; exists {
		t.Error("Ports were not removed from main service")
	}

	// Check dependencies were added
	deps, exists := webapp["depends_on"]
	if !exists {
		t.Error("depends_on was not added")
	}
	depsList, ok := deps.([]interface{})
	if !ok || len(depsList) == 0 {
		t.Error("depends_on is not a list or is empty")
	}
	found := false
	for _, dep := range depsList {
		if dep == "tailscale" {
			found = true
			break
		}
	}
	if !found {
		t.Error("tailscale not found in depends_on")
	}

	// Check Tailscale service configuration
	tailscale, ok := compose.Services["tailscale"].(map[string]interface{})
	if !ok {
		t.Fatal("tailscale service not found or wrong type")
	}

	// Check image
	if image, exists := tailscale["image"]; !exists || image != "tailscale/tailscale:latest" {
		t.Errorf("Tailscale image not set correctly, got: %v", tailscale["image"])
	}

	// Check hostname
	if hostname, exists := tailscale["hostname"]; !exists || hostname != "testhost" {
		t.Errorf("Tailscale hostname not set correctly, got: %v", tailscale["hostname"])
	}

	// Check volumes
	volumes, exists := tailscale["volumes"]
	if !exists {
		t.Error("Tailscale volumes not set")
	}
	volumesList, ok := volumes.([]interface{})
	if !ok || len(volumesList) != 2 {
		t.Errorf("Tailscale volumes incorrect, got: %v", volumes)
	}

	// Check environment variables
	env, exists := tailscale["environment"]
	if !exists {
		t.Error("Tailscale environment not set")
	}
	envList, ok := env.([]interface{})
	if !ok || len(envList) == 0 {
		t.Error("Tailscale environment is not a list or is empty")
	}

	// Check for required environment variables
	hasAuthKey := false
	hasHostname := false
	for _, e := range envList {
		envStr, ok := e.(string)
		if !ok {
			continue
		}
		if strings.Contains(envStr, "TS_AUTHKEY") {
			hasAuthKey = true
		}
		if strings.Contains(envStr, "TS_HOSTNAME=testhost") {
			hasHostname = true
		}
	}
	if !hasAuthKey {
		t.Error("TS_AUTHKEY not found in environment")
	}
	if !hasHostname {
		t.Error("TS_HOSTNAME not set correctly in environment")
	}
}

func TestRemoveTailscaleSidecar(t *testing.T) {
	// Create a compose file with Tailscale sidecar already added
	compose := &ComposeFile{
		Version: "3.8",
		Services: map[string]interface{}{
			"tailscale": map[string]interface{}{
				"image":    "tailscale/tailscale:latest",
				"hostname": "testhost",
			},
			"webapp": map[string]interface{}{
				"image":           "nginx:alpine",
				"network_mode":    "service:tailscale",
				"x-original-ports": []interface{}{"8080:80"},
				"depends_on":      []interface{}{"tailscale", "db"},
			},
			"db": map[string]interface{}{
				"image": "postgres:13",
			},
		},
	}

	// Remove Tailscale sidecar
	err := RemoveTailscaleSidecar(compose)
	if err != nil {
		t.Fatalf("Failed to remove Tailscale sidecar: %v", err)
	}

	// Check that Tailscale service was removed
	if _, exists := compose.Services["tailscale"]; exists {
		t.Error("Tailscale service was not removed")
	}

	// Check that main service was restored
	webapp, ok := compose.Services["webapp"].(map[string]interface{})
	if !ok {
		t.Fatal("webapp service not found or wrong type")
	}

	// Check network_mode was removed
	if _, exists := webapp["network_mode"]; exists {
		t.Error("network_mode was not removed")
	}

	// Check ports were restored
	ports, exists := webapp["ports"]
	if !exists {
		t.Error("Ports were not restored")
	}
	portsList, ok := ports.([]interface{})
	if !ok || len(portsList) == 0 {
		t.Error("Ports were not restored correctly")
	}

	// Check x-original-ports was removed
	if _, exists := webapp["x-original-ports"]; exists {
		t.Error("x-original-ports was not removed")
	}

	// Check tailscale was removed from dependencies
	deps, exists := webapp["depends_on"]
	if !exists {
		t.Error("depends_on was removed entirely")
	}
	depsList, ok := deps.([]interface{})
	if !ok {
		t.Error("depends_on is not a list")
	}
	for _, dep := range depsList {
		if dep == "tailscale" {
			t.Error("tailscale was not removed from depends_on")
		}
	}
	// Check that other dependencies remain
	found := false
	for _, dep := range depsList {
		if dep == "db" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Other dependencies were removed")
	}
}

func TestHasTailscaleSidecar(t *testing.T) {
	tests := []struct {
		name     string
		compose  *ComposeFile
		expected bool
	}{
		{
			name: "Has Tailscale sidecar",
			compose: &ComposeFile{
				Services: map[string]interface{}{
					"tailscale": map[string]interface{}{
						"image": "tailscale/tailscale:latest",
					},
					"webapp": map[string]interface{}{
						"image": "nginx:alpine",
					},
				},
			},
			expected: true,
		},
		{
			name: "No Tailscale sidecar",
			compose: &ComposeFile{
				Services: map[string]interface{}{
					"webapp": map[string]interface{}{
						"image": "nginx:alpine",
					},
				},
			},
			expected: false,
		},
		{
			name:     "No services",
			compose:  &ComposeFile{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasTailscaleSidecar(tt.compose)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetMainServiceName(t *testing.T) {
	tests := []struct {
		name     string
		compose  *ComposeFile
		expected string
	}{
		{
			name: "Single service",
			compose: &ComposeFile{
				Services: map[string]interface{}{
					"webapp": map[string]interface{}{
						"image": "nginx:alpine",
					},
				},
			},
			expected: "webapp",
		},
		{
			name: "Multiple services without tailscale",
			compose: &ComposeFile{
				Services: map[string]interface{}{
					"webapp": map[string]interface{}{
						"image": "nginx:alpine",
					},
					"db": map[string]interface{}{
						"image": "postgres:13",
					},
				},
			},
			expected: "webapp", // Returns first one found
		},
		{
			name: "Services with tailscale",
			compose: &ComposeFile{
				Services: map[string]interface{}{
					"tailscale": map[string]interface{}{
						"image": "tailscale/tailscale:latest",
					},
					"webapp": map[string]interface{}{
						"image": "nginx:alpine",
					},
				},
			},
			expected: "webapp", // Should skip tailscale
		},
		{
			name:     "No services",
			compose:  &ComposeFile{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMainServiceName(tt.compose)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestModifyComposeForTailscale(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	appPath := tmpDir

	// Create a basic docker-compose.yml
	composeContent := `version: '3.8'
services:
  webapp:
    image: nginx:alpine
    ports:
      - "8080:80"
`
	composePath := filepath.Join(appPath, "docker-compose.yml")
	err := os.WriteFile(composePath, []byte(composeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	// Modify compose for Tailscale
	err = ModifyComposeForTailscale(appPath, "testapp", "testhost", "tskey-auth-test")
	if err != nil {
		t.Fatalf("Failed to modify compose for Tailscale: %v", err)
	}

	// Check that .env file was created
	envPath := filepath.Join(appPath, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Error(".env file was not created")
	}

	// Check .env content
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}
	if !strings.Contains(string(envContent), "TS_AUTHKEY=tskey-auth-test") {
		t.Error(".env file does not contain auth key")
	}

	// Check that compose file was modified
	modifiedContent, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("Failed to read modified compose file: %v", err)
	}

	// Parse the modified compose file
	var compose map[string]interface{}
	err = yaml.Unmarshal(modifiedContent, &compose)
	if err != nil {
		t.Fatalf("Failed to parse modified compose file: %v", err)
	}

	// Check that tailscale service exists
	services, ok := compose["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Services not found in compose file")
	}
	if _, exists := services["tailscale"]; !exists {
		t.Error("Tailscale service not found in modified compose file")
	}
}

func TestRestoreComposeFromTailscale(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	appPath := tmpDir

	// Create a compose file with Tailscale sidecar
	composeContent := `version: '3.8'
services:
  tailscale:
    image: tailscale/tailscale:latest
    hostname: testhost
  webapp:
    image: nginx:alpine
    network_mode: service:tailscale
    x-original-ports:
      - "8080:80"
`
	composePath := filepath.Join(appPath, "docker-compose.yml")
	err := os.WriteFile(composePath, []byte(composeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	// Create .env file
	envPath := filepath.Join(appPath, ".env")
	err = os.WriteFile(envPath, []byte("TS_AUTHKEY=test"), 0644)
	if err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Create fake state files
	stateFiles := []string{"tailscaled.state", "tailscaled.sock", "tailscaled.log"}
	for _, file := range stateFiles {
		filePath := filepath.Join(appPath, file)
		err = os.WriteFile(filePath, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to write state file %s: %v", file, err)
		}
	}

	// Restore compose from Tailscale
	err = RestoreComposeFromTailscale(appPath)
	if err != nil {
		t.Fatalf("Failed to restore compose from Tailscale: %v", err)
	}

	// Check that .env file was removed
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Error(".env file was not removed")
	}

	// Check that state files were removed
	for _, file := range stateFiles {
		filePath := filepath.Join(appPath, file)
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("State file %s was not removed", file)
		}
	}

	// Check that compose file was restored
	restoredContent, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("Failed to read restored compose file: %v", err)
	}

	// Parse the restored compose file
	var compose map[string]interface{}
	err = yaml.Unmarshal(restoredContent, &compose)
	if err != nil {
		t.Fatalf("Failed to parse restored compose file: %v", err)
	}

	// Check that tailscale service was removed
	services, ok := compose["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Services not found in compose file")
	}
	if _, exists := services["tailscale"]; exists {
		t.Error("Tailscale service was not removed from compose file")
	}

	// Check that webapp service was restored
	webapp, ok := services["webapp"].(map[string]interface{})
	if !ok {
		t.Fatal("webapp service not found")
	}
	if _, exists := webapp["network_mode"]; exists {
		t.Error("network_mode was not removed from webapp")
	}
	if _, exists := webapp["x-original-ports"]; exists {
		t.Error("x-original-ports was not removed from webapp")
	}
}