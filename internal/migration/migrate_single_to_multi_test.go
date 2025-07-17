package migration

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

func TestGenerateComposeFromContainer(t *testing.T) {
	tests := []struct {
		name          string
		appName       string
		containerInfo container.InspectResponse
		wantServices  bool
		wantPorts     bool
		wantVolumes   bool
		wantEnv       bool
	}{
		{
			name:    "basic container with ports",
			appName: "myapp",
			containerInfo: container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						PortBindings: nat.PortMap{
							"80/tcp": []nat.PortBinding{
								{HostPort: "8080"},
							},
						},
						RestartPolicy: container.RestartPolicy{
							Name: "unless-stopped",
						},
					},
				},
				Config: &container.Config{
					Image: "nginx:latest",
					Env: []string{
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
						"NGINX_VERSION=1.21.0",
					},
				},
			},
			wantServices: true,
			wantPorts:    true,
			wantEnv:      true,
		},
		{
			name:    "container with volumes",
			appName: "webapp",
			containerInfo: container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						Binds: []string{
							"./data:/var/lib/postgresql/data",
							"/opt/ontree/apps/webapp/config:/etc/postgresql",
						},
					},
				},
				Config: &container.Config{
					Image: "postgres:13",
				},
			},
			wantServices: true,
			wantVolumes:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compose := generateComposeFromContainer(tt.appName, tt.containerInfo)

			// Check basic structure
			if _, ok := compose["version"]; !ok {
				t.Error("compose missing version")
			}

			services, ok := compose["services"].(map[string]interface{})
			if !ok || len(services) == 0 {
				if tt.wantServices {
					t.Error("compose missing services")
				}
				return
			}

			// Check app service
			appService, ok := services["app"].(map[string]interface{})
			if !ok {
				t.Error("compose missing app service")
				return
			}

			// Check image
			if appService["image"] != tt.containerInfo.Config.Image {
				t.Errorf("wrong image: got %v, want %v", appService["image"], tt.containerInfo.Config.Image)
			}

			// Check container name
			expectedName := "ontree-" + tt.appName + "-app-1"
			if appService["container_name"] != expectedName {
				t.Errorf("wrong container name: got %v, want %v", appService["container_name"], expectedName)
			}

			// Check ports
			if tt.wantPorts {
				if _, ok := appService["ports"]; !ok {
					t.Error("service missing ports")
				}
			}

			// Check volumes
			if tt.wantVolumes {
				if _, ok := appService["volumes"]; !ok {
					t.Error("service missing volumes")
				}
			}

			// Check environment
			if tt.wantEnv {
				if envs, ok := appService["environment"].([]string); ok {
					// Should filter out PATH but keep NGINX_VERSION
					hasNginxVersion := false
					hasPath := false
					for _, env := range envs {
						if env == "NGINX_VERSION=1.21.0" {
							hasNginxVersion = true
						}
						if env == "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" {
							hasPath = true
						}
					}
					if !hasNginxVersion {
						t.Error("NGINX_VERSION should be included")
					}
					if hasPath {
						t.Error("PATH should be filtered out")
					}
				}
			}
		})
	}
}

func TestGenerateEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []string
		wantEnv  string
		wantSkip bool
	}{
		{
			name: "filter secrets",
			envVars: []string{
				"PATH=/usr/bin",
				"API_KEY=secret123",
				"DB_PASSWORD=mypass",
				"NORMAL_VAR=value",
				"SECRET_TOKEN=abc123",
			},
			wantEnv: "API_KEY=secret123\nDB_PASSWORD=mypass\nSECRET_TOKEN=abc123",
		},
		{
			name: "no secrets",
			envVars: []string{
				"PATH=/usr/bin",
				"APP_VERSION=1.0.0",
				"DEBUG=true",
			},
			wantEnv: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateEnvFile(tt.envVars)
			if result != tt.wantEnv {
				t.Errorf("generateEnvFile() = %q, want %q", result, tt.wantEnv)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice []string
		item  string
		want  bool
	}{
		{[]string{"app1", "app2", "app3"}, "app2", true},
		{[]string{"app1", "app2", "app3"}, "app4", false},
		{[]string{}, "app1", false},
		{nil, "app1", false},
	}

	for _, tt := range tests {
		got := contains(tt.slice, tt.item)
		if got != tt.want {
			t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.want)
		}
	}
}