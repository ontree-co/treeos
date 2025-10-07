// Package security provides security validation for docker-compose.yml files.
package security

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"treeos/internal/config"
)

// DangerousCapabilities defines the list of container capabilities that are not allowed
var DangerousCapabilities = []string{
	"SYS_ADMIN",
	"NET_ADMIN",
	"SYS_MODULE",
	"SYS_RAWIO",
	"SYS_PTRACE",
	"SYS_BOOT",
	"MAC_ADMIN",
	"MAC_OVERRIDE",
	"DAC_READ_SEARCH",
	"SETFCAP",
}

// ComposeConfig represents a minimal docker-compose.yml structure for validation
type ComposeConfig struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig represents a service configuration in docker-compose.yml
type ServiceConfig struct {
	Privileged    bool          `yaml:"privileged"`
	CapAdd        []string      `yaml:"cap_add"`
	Volumes       []interface{} `yaml:"volumes"`
	Build         interface{}   `yaml:"build"`
	Image         string        `yaml:"image"`
	Environment   interface{}   `yaml:"environment"`
	Ports         []interface{} `yaml:"ports"`
	Networks      interface{}   `yaml:"networks"`
	RestartPolicy string        `yaml:"restart"`
	Command       interface{}   `yaml:"command"`
	Entrypoint    interface{}   `yaml:"entrypoint"`
	WorkingDir    string        `yaml:"working_dir"`
	User          string        `yaml:"user"`
	ExtraHosts    []string      `yaml:"extra_hosts"`
	DependsOn     interface{}   `yaml:"depends_on"`
	Deploy        interface{}   `yaml:"deploy"`
}

// ValidationError represents a security validation error
type ValidationError struct {
	Service string
	Rule    string
	Detail  string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("security validation failed for service '%s': %s - %s", e.Service, e.Rule, e.Detail)
}

// Validator handles security validation of docker-compose configurations
type Validator struct {
	appName string
}

// NewValidator creates a new security validator for the given app
func NewValidator(appName string) *Validator {
	return &Validator{
		appName: appName,
	}
}

// ValidateCompose validates a docker-compose.yml content against security rules
func (v *Validator) ValidateCompose(yamlContent []byte) error {
	var config ComposeConfig

	// Parse YAML
	if err := yaml.Unmarshal(yamlContent, &config); err != nil {
		return fmt.Errorf("failed to parse docker-compose.yml: %w", err)
	}

	// Validate each service
	for serviceName, service := range config.Services {
		// Check privileged mode
		if err := v.validatePrivilegedMode(serviceName, service); err != nil {
			return err
		}

		// Check capabilities
		if err := v.validateCapabilities(serviceName, service); err != nil {
			return err
		}

		// Check bind mounts
		if err := v.validateBindMounts(serviceName, service); err != nil {
			return err
		}
	}

	return nil
}

// validatePrivilegedMode checks if privileged mode is disabled
func (v *Validator) validatePrivilegedMode(serviceName string, service ServiceConfig) error {
	if service.Privileged {
		return ValidationError{
			Service: serviceName,
			Rule:    "privileged mode",
			Detail:  "privileged mode is not allowed for security reasons",
		}
	}
	return nil
}

// validateCapabilities checks for dangerous container capabilities
func (v *Validator) validateCapabilities(serviceName string, service ServiceConfig) error {
	for _, cap := range service.CapAdd {
		// Normalize capability name (remove CAP_ prefix if present)
		normalizedCap := strings.TrimPrefix(strings.ToUpper(cap), "CAP_")

		for _, dangerous := range DangerousCapabilities {
			if normalizedCap == dangerous {
				return ValidationError{
					Service: serviceName,
					Rule:    "dangerous capabilities",
					Detail:  fmt.Sprintf("capability '%s' is not allowed for security reasons", cap),
				}
			}
		}
	}
	return nil
}

// validateBindMounts checks that all bind mounts follow the required path structure
func (v *Validator) validateBindMounts(serviceName string, service ServiceConfig) error {
	// In demo mode, allow relative paths
	isDemo := os.Getenv("TREEOS_RUN_MODE") == "demo"

	// Define allowed paths for volumes and mounts
	var volumesPath, mntPath string
	if isDemo {
		// In demo mode, paths are relative to the docker-compose.yml location
		volumesPath = "./volumes/"
		mntPath = "./mnt/"
	} else {
		// In production mode, use OS-specific absolute paths
		volumesPath = fmt.Sprintf("%s/", config.GetAppVolumesPath(v.appName))
		mntPath = fmt.Sprintf("%s/", config.GetAppMntPath(v.appName))
	}

	for _, volume := range service.Volumes {
		// Volumes can be strings (bind mounts) or maps (named volumes)
		switch v := volume.(type) {
		case string:
			// Check if it's a bind mount (contains ':')
			if strings.Contains(v, ":") {
				parts := strings.SplitN(v, ":", 3)
				if len(parts) >= 2 {
					hostPath := parts[0]

					// Skip named volumes (don't start with / or .)
					if !strings.HasPrefix(hostPath, "/") && !strings.HasPrefix(hostPath, ".") {
						continue
					}

					// Normalize path
					hostPath = strings.TrimSuffix(hostPath, "/")

					// Check allowed paths based on mode
					if isDemo {
						// Demo mode: only allow relative paths
						if !strings.HasPrefix(hostPath, ".") {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' must be a relative path in demo mode", hostPath),
							}
						}

						// Check if path is in one of the allowed directories
						// Allow ./volumes/, ./mnt/, and ../../shared/ (relative parent path for shared)
						if !strings.HasPrefix(hostPath, volumesPath) &&
						   !strings.HasPrefix(hostPath, mntPath) &&
						   !strings.HasPrefix(hostPath, "../../shared/") &&
						   !strings.HasPrefix(hostPath, "./shared/") {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' is not allowed. Use paths within '%s', '%s', or '../../shared/'",
									hostPath, volumesPath, mntPath),
							}
						}
					} else {
						// Production mode: only allow absolute paths
						if !strings.HasPrefix(hostPath, "/") {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' must be an absolute path in production mode", hostPath),
							}
						}

						// Check if path is in one of the allowed directories
						sharedPath := fmt.Sprintf("%s/", config.GetSharedPath())
						if !strings.HasPrefix(hostPath, volumesPath) &&
						   !strings.HasPrefix(hostPath, mntPath) &&
						   !strings.HasPrefix(hostPath, sharedPath) {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' is not allowed. Use paths within '%s', '%s', or '%s'",
									hostPath, volumesPath, mntPath, sharedPath),
							}
						}
					}
				}
			}
		case map[string]interface{}:
			// Handle long-form volume syntax
			if source, ok := v["source"].(string); ok {
				if volumeType, ok := v["type"].(string); ok && volumeType == "bind" {
					// Skip named volumes
					if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") {
						continue
					}

					// Normalize path
					source = strings.TrimSuffix(source, "/")

					// Check allowed paths based on mode
					if isDemo {
						// Demo mode: only allow relative paths
						if !strings.HasPrefix(source, ".") {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' must be a relative path in demo mode", source),
							}
						}

						// Check if path is in one of the allowed directories
						// Allow ./volumes/, ./mnt/, and ../../shared/ (relative parent path for shared)
						if !strings.HasPrefix(source, volumesPath) &&
						   !strings.HasPrefix(source, mntPath) &&
						   !strings.HasPrefix(source, "../../shared/") &&
						   !strings.HasPrefix(source, "./shared/") {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' is not allowed. Use paths within '%s', '%s', or '../../shared/'",
									source, volumesPath, mntPath),
							}
						}
					} else {
						// Production mode: only allow absolute paths
						if !strings.HasPrefix(source, "/") {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' must be an absolute path in production mode", source),
							}
						}

						// Check if path is in one of the allowed directories
						sharedPath := fmt.Sprintf("%s/", config.GetSharedPath())
						if !strings.HasPrefix(source, volumesPath) &&
						   !strings.HasPrefix(source, mntPath) &&
						   !strings.HasPrefix(source, sharedPath) {
							return ValidationError{
								Service: serviceName,
								Rule:    "bind mount path",
								Detail:  fmt.Sprintf("bind mount path '%s' is not allowed. Use paths within '%s', '%s', or '%s'",
									source, volumesPath, mntPath, sharedPath),
							}
						}
					}
				}
			}
		}
	}

	return nil
}
