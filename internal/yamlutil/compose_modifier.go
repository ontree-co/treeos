package yamlutil

import (
	"fmt"
	"os"
	"strings"
)

// AddTailscaleSidecar adds a Tailscale sidecar container to the compose file
// and modifies the main service to use the Tailscale network
func AddTailscaleSidecar(compose *ComposeFile, appName, hostname, authKey string) error {
	if compose.Services == nil {
		compose.Services = make(map[string]interface{})
	}

	// Find the main service (first non-tailscale service)
	mainService := GetMainServiceName(compose)
	if mainService == "" {
		return fmt.Errorf("no main service found in compose file")
	}

	// Create Tailscale sidecar service
	tailscaleService := map[string]interface{}{
		"image":          "tailscale/tailscale:latest",
		"container_name": fmt.Sprintf("%s-tailscale", appName),
		"hostname":       hostname,
		"volumes": []interface{}{
			"./:/var/lib/tailscale",
			"/dev/net/tun:/dev/net/tun",
		},
		"cap_add": []interface{}{
			"net_admin",
			"net_raw",
		},
		"environment": []interface{}{
			"TS_AUTHKEY=${TS_AUTHKEY}",
			"TS_STATE_DIR=/var/lib/tailscale",
			"TS_USERSPACE=false",
			fmt.Sprintf("TS_HOSTNAME=%s", hostname),
		},
		"restart": "unless-stopped",
	}

	// Add Tailscale service
	compose.Services["tailscale"] = tailscaleService

	// Modify main service to use Tailscale network
	if mainServiceMap, ok := compose.Services[mainService].(map[string]interface{}); ok {
		// Store original port mapping if it exists
		if ports, exists := mainServiceMap["ports"]; exists {
			// Store original ports in a comment or metadata for restoration
			mainServiceMap["x-original-ports"] = ports
			// Remove ports since we're using Tailscale network
			delete(mainServiceMap, "ports")
		}

		// Set network mode to use Tailscale container
		mainServiceMap["network_mode"] = "service:tailscale"

		// Add dependency on Tailscale container
		if deps, exists := mainServiceMap["depends_on"]; exists {
			// Append to existing dependencies
			switch v := deps.(type) {
			case []interface{}:
				v = append(v, "tailscale")
				mainServiceMap["depends_on"] = v
			case []string:
				mainServiceMap["depends_on"] = append(v, "tailscale")
			default:
				mainServiceMap["depends_on"] = []interface{}{deps, "tailscale"}
			}
		} else {
			mainServiceMap["depends_on"] = []interface{}{"tailscale"}
		}
	}

	return nil
}

// RemoveTailscaleSidecar removes the Tailscale sidecar container from the compose file
// and restores the original network configuration
func RemoveTailscaleSidecar(compose *ComposeFile) error {
	if compose.Services == nil {
		return fmt.Errorf("no services found in compose file")
	}

	// Remove Tailscale service
	delete(compose.Services, "tailscale")

	// Find and restore main service
	for _, service := range compose.Services {
		if serviceMap, ok := service.(map[string]interface{}); ok {
			// Check if this service was using Tailscale network
			if networkMode, exists := serviceMap["network_mode"]; exists {
				if modeStr, ok := networkMode.(string); ok && modeStr == "service:tailscale" {
					// Remove network_mode
					delete(serviceMap, "network_mode")

					// Restore original ports if they exist
					if originalPorts, exists := serviceMap["x-original-ports"]; exists {
						serviceMap["ports"] = originalPorts
						delete(serviceMap, "x-original-ports")
					}

					// Remove tailscale from dependencies
					if deps, exists := serviceMap["depends_on"]; exists {
						switch v := deps.(type) {
						case []interface{}:
							newDeps := []interface{}{}
							for _, dep := range v {
								if depStr, ok := dep.(string); !ok || depStr != "tailscale" {
									newDeps = append(newDeps, dep)
								}
							}
							if len(newDeps) > 0 {
								serviceMap["depends_on"] = newDeps
							} else {
								delete(serviceMap, "depends_on")
							}
						case []string:
							newDeps := []string{}
							for _, dep := range v {
								if dep != "tailscale" {
									newDeps = append(newDeps, dep)
								}
							}
							if len(newDeps) > 0 {
								serviceMap["depends_on"] = newDeps
							} else {
								delete(serviceMap, "depends_on")
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// GetMainServiceName returns the name of the main service (first non-tailscale service)
func GetMainServiceName(compose *ComposeFile) string {
	if compose.Services == nil {
		return ""
	}

	// Return first service that's not tailscale
	for name := range compose.Services {
		if name != "tailscale" {
			return name
		}
	}

	return ""
}

// HasTailscaleSidecar checks if the compose file already has a Tailscale sidecar
func HasTailscaleSidecar(compose *ComposeFile) bool {
	if compose.Services == nil {
		return false
	}

	_, exists := compose.Services["tailscale"]
	return exists
}

// ExtractOriginalPorts extracts the original port mappings from the main service
// This is useful for restoring ports when removing Tailscale
func ExtractOriginalPorts(compose *ComposeFile) []string {
	mainService := GetMainServiceName(compose)
	if mainService == "" {
		return nil
	}

	if serviceMap, ok := compose.Services[mainService].(map[string]interface{}); ok {
		// Check for stored original ports first
		if originalPorts, exists := serviceMap["x-original-ports"]; exists {
			return convertToStringSlice(originalPorts)
		}

		// Otherwise check current ports
		if ports, exists := serviceMap["ports"]; exists {
			return convertToStringSlice(ports)
		}
	}

	return nil
}

// convertToStringSlice converts various port formats to string slice
func convertToStringSlice(ports interface{}) []string {
	var result []string

	switch v := ports.(type) {
	case []interface{}:
		for _, port := range v {
			if portStr, ok := port.(string); ok {
				result = append(result, portStr)
			} else if portMap, ok := port.(map[string]interface{}); ok {
				// Handle long-form port syntax
				if published, ok := portMap["published"]; ok {
					if target, ok := portMap["target"]; ok {
						result = append(result, fmt.Sprintf("%v:%v", published, target))
					}
				}
			}
		}
	case []string:
		result = v
	case string:
		result = []string{v}
	}

	return result
}

// CreateEnvFile creates a .env file with the Tailscale auth key
func CreateEnvFile(appPath, authKey string) error {
	envContent := fmt.Sprintf("TS_AUTHKEY=%s\n", authKey)
	envPath := fmt.Sprintf("%s/.env", strings.TrimSuffix(appPath, "/"))

	// Write .env file with restrictive permissions
	lock := getFileLock(envPath)
	lock.Lock()
	defer lock.Unlock()

	if err := writeFileWithPermissions(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	return nil
}

// writeFileWithPermissions writes a file with specific permissions
func writeFileWithPermissions(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// ModifyComposeForTailscale reads, modifies, and writes back a compose file with Tailscale sidecar
func ModifyComposeForTailscale(appPath, appName, hostname, authKey string) error {
	composePath := fmt.Sprintf("%s/docker-compose.yml", strings.TrimSuffix(appPath, "/"))

	// Read existing compose file
	compose, err := ReadComposeWithMetadata(composePath)
	if err != nil {
		return fmt.Errorf("failed to read compose file: %w", err)
	}

	// Add Tailscale sidecar
	if err := AddTailscaleSidecar(compose, appName, hostname, authKey); err != nil {
		return fmt.Errorf("failed to add Tailscale sidecar: %w", err)
	}

	// Write modified compose file
	if err := WriteComposeWithMetadata(composePath, compose); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Create .env file with auth key
	if err := CreateEnvFile(appPath, authKey); err != nil {
		return fmt.Errorf("failed to create .env file: %w", err)
	}

	return nil
}

// RestoreComposeFromTailscale removes Tailscale modifications from a compose file
func RestoreComposeFromTailscale(appPath string) error {
	composePath := fmt.Sprintf("%s/docker-compose.yml", strings.TrimSuffix(appPath, "/"))

	// Read existing compose file
	compose, err := ReadComposeWithMetadata(composePath)
	if err != nil {
		return fmt.Errorf("failed to read compose file: %w", err)
	}

	// Remove Tailscale sidecar
	if err := RemoveTailscaleSidecar(compose); err != nil {
		return fmt.Errorf("failed to remove Tailscale sidecar: %w", err)
	}

	// Write modified compose file
	if err := WriteComposeWithMetadata(composePath, compose); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Remove .env file
	envPath := fmt.Sprintf("%s/.env", strings.TrimSuffix(appPath, "/"))
	os.Remove(envPath) // Ignore error if file doesn't exist

	// Clean up Tailscale state files
	stateFiles := []string{
		"tailscaled.state",
		"tailscaled.sock",
		"tailscaled.log",
	}
	for _, file := range stateFiles {
		filePath := fmt.Sprintf("%s/%s", strings.TrimSuffix(appPath, "/"), file)
		os.Remove(filePath) // Ignore error if file doesn't exist
	}

	return nil
}
