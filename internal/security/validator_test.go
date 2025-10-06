package security

import (
	"os"
	"strings"
	"testing"
)

func TestValidateCompose_ValidConfiguration(t *testing.T) {
	validator := NewValidator("test-app")

	// Valid configuration with proper bind mounts and no security issues
	yamlContent := `
version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    volumes:
      - /opt/ontree/apps/test-app/volumes/web_data:/usr/share/nginx/html
      - nginx-config:/etc/nginx
  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
    volumes:
      - /opt/ontree/apps/test-app/volumes/db_data:/var/lib/postgresql/data
      - db-backup:/backup
volumes:
  nginx-config:
  db-backup:
`

	err := validator.ValidateCompose([]byte(yamlContent))
	if err != nil {
		t.Errorf("Expected valid configuration to pass validation, got error: %v", err)
	}
}

func TestValidateCompose_InvalidYAML(t *testing.T) {
	validator := NewValidator("test-app")

	yamlContent := `
version: '3.8'
services:
  web:
    image: nginx:latest
    invalid yaml here
`

	err := validator.ValidateCompose([]byte(yamlContent))
	if err == nil {
		t.Error("Expected invalid YAML to fail validation")
	}
	if !strings.Contains(err.Error(), "failed to parse docker-compose.yml") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

func TestValidatePrivilegedMode(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		shouldFail  bool
		errorMsg    string
	}{
		{
			name: "service with privileged mode enabled",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    privileged: true
`,
			shouldFail: true,
			errorMsg:   "privileged mode is not allowed",
		},
		{
			name: "service with privileged mode disabled",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    privileged: false
`,
			shouldFail: false,
		},
		{
			name: "service without privileged mode specified",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
`,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("test-app")
			err := validator.ValidateCompose([]byte(tt.yamlContent))

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail but it passed")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass but got error: %v", err)
				}
			}
		})
	}
}

func TestValidateCapabilities(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		shouldFail  bool
		errorMsg    string
	}{
		{
			name: "service with dangerous capability SYS_ADMIN",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    cap_add:
      - SYS_ADMIN
`,
			shouldFail: true,
			errorMsg:   "capability 'SYS_ADMIN' is not allowed",
		},
		{
			name: "service with dangerous capability NET_ADMIN",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    cap_add:
      - NET_ADMIN
`,
			shouldFail: true,
			errorMsg:   "capability 'NET_ADMIN' is not allowed",
		},
		{
			name: "service with CAP_ prefix (should still be caught)",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    cap_add:
      - CAP_SYS_ADMIN
`,
			shouldFail: true,
			errorMsg:   "capability 'CAP_SYS_ADMIN' is not allowed",
		},
		{
			name: "service with allowed capabilities",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    cap_add:
      - CHOWN
      - SETUID
      - SETGID
`,
			shouldFail: false,
		},
		{
			name: "service with mixed case dangerous capability",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    cap_add:
      - sys_admin
`,
			shouldFail: true,
			errorMsg:   "capability 'sys_admin' is not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("test-app")
			err := validator.ValidateCompose([]byte(tt.yamlContent))

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail but it passed")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass but got error: %v", err)
				}
			}
		})
	}
}

func TestValidateBindMounts(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		yamlContent string
		shouldFail  bool
		errorMsg    string
	}{
		{
			name:    "valid bind mount in volumes directory",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/my-app/volumes/web_data:/usr/share/nginx/html
`,
			shouldFail: false,
		},
		{
			name:    "valid bind mount in mnt directory",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/my-app/mnt/config:/etc/nginx/conf.d
`,
			shouldFail: false,
		},
		{
			name:    "valid bind mount in shared directory",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/shared/models:/models:ro
`,
			shouldFail: false,
		},
		{
			name:    "bind mount outside allowed directory",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /etc/passwd:/etc/passwd:ro
`,
			shouldFail: true,
			errorMsg:   "is not allowed",
		},
		{
			name:    "bind mount in wrong app directory",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/other-app/volumes/data:/data
`,
			shouldFail: true,
			errorMsg:   "is not allowed",
		},
		{
			name:    "bind mount with trailing slash",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/my-app/volumes/data/:/usr/share/nginx/html
`,
			shouldFail: false,
		},
		{
			name:    "named volume (should be allowed)",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - web-data:/usr/share/nginx/html
volumes:
  web-data:
`,
			shouldFail: false,
		},
		{
			name:    "relative path bind mount not allowed in production",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - ./data:/usr/share/nginx/html
`,
			shouldFail: true,
			errorMsg:   "must be an absolute path in production mode",
		},
		{
			name:    "shared ollama relative path not allowed in production",
			appName: "ollama-cpu",
			yamlContent: `
version: '3.8'
services:
  ollama:
    image: ollama/ollama:latest
    volumes:
      - ./shared/ollama:/root/.ollama
`,
			shouldFail: true,
			errorMsg:   "must be an absolute path in production mode",
		},
		{
			name:    "long-form volume syntax with bind mount",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - type: bind
        source: /opt/ontree/apps/my-app/mnt/config
        target: /etc/nginx
`,
			shouldFail: false,
		},
		{
			name:    "long-form volume syntax with invalid path",
			appName: "my-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - type: bind
        source: /var/log
        target: /logs
`,
			shouldFail: true,
			errorMsg:   "is not allowed",
		},
		{
			name:    "mixed volume types",
			appName: "test-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/test-app/volumes/html:/usr/share/nginx/html
      - nginx-cache:/var/cache/nginx
      - type: volume
        source: nginx-logs
        target: /var/log/nginx
volumes:
  nginx-cache:
  nginx-logs:
`,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator(tt.appName)
			err := validator.ValidateCompose([]byte(tt.yamlContent))

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail but it passed")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass but got error: %v", err)
				}
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError{
		Service: "web",
		Rule:    "privileged mode",
		Detail:  "privileged mode is not allowed for security reasons",
	}

	expected := "security validation failed for service 'web': privileged mode - privileged mode is not allowed for security reasons"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestComplexScenarios(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		yamlContent string
		shouldFail  bool
		errorMsg    string
	}{
		{
			name:    "multiple services with mixed violations",
			appName: "complex-app",
			yamlContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/complex-app/volumes/html:/usr/share/nginx/html
  db:
    image: postgres:15
    privileged: true
    volumes:
      - /opt/ontree/apps/complex-app/volumes/db_data:/var/lib/postgresql/data
`,
			shouldFail: true,
			errorMsg:   "privileged mode is not allowed",
		},
		{
			name:    "service with multiple issues",
			appName: "multi-issue",
			yamlContent: `
version: '3.8'
services:
  app:
    image: custom:latest
    privileged: true
    cap_add:
      - SYS_ADMIN
    volumes:
      - /etc/shadow:/etc/shadow
`,
			shouldFail: true,
			errorMsg:   "privileged mode is not allowed", // Should fail on first issue
		},
		{
			name:    "all dangerous capabilities",
			appName: "cap-test",
			yamlContent: `
version: '3.8'
services:
  scanner:
    image: security-scanner:latest
    cap_add:
      - SYS_MODULE
      - SYS_RAWIO
      - SYS_PTRACE
      - MAC_ADMIN
      - SETFCAP
`,
			shouldFail: true,
			errorMsg:   "is not allowed for security reasons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator(tt.appName)
			err := validator.ValidateCompose([]byte(tt.yamlContent))

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail but it passed")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass but got error: %v", err)
				}
			}
		})
	}
}

func TestValidateBindMounts_DemoMode(t *testing.T) {
	// Set demo mode environment variable
	oldEnv := os.Getenv("TREEOS_RUN_MODE")
	os.Setenv("TREEOS_RUN_MODE", "demo")              //nolint:errcheck,gosec // Test setup
	defer os.Setenv("TREEOS_RUN_MODE", oldEnv) //nolint:errcheck // Test cleanup

	tests := []struct {
		name        string
		appName     string
		yamlContent string
		shouldFail  bool
		errorMsg    string
	}{
		{
			name:    "valid bind mount in volumes directory (demo)",
			appName: "test-app",
			yamlContent: `
version: "3.8"
services:
  web:
    image: nginx:latest
    volumes:
      - ./volumes/web_data:/usr/share/nginx/html
`,
			shouldFail: false,
		},
		{
			name:    "valid bind mount in mnt directory (demo)",
			appName: "test-app",
			yamlContent: `
version: "3.8"
services:
  web:
    image: nginx:latest
    volumes:
      - ./mnt/config:/etc/nginx/conf.d
`,
			shouldFail: false,
		},
		{
			name:    "shared ollama path in demo mode",
			appName: "test-app",
			yamlContent: `
version: "3.8"
services:
  ollama:
    image: ollama/ollama:latest
    volumes:
      - ./shared/ollama:/root/.ollama
`,
			shouldFail: false,
		},
		{
			name:    "absolute path not allowed in demo mode",
			appName: "test-app",
			yamlContent: `
version: "3.8"
services:
  web:
    image: nginx:latest
    volumes:
      - /opt/ontree/apps/test-app/volumes/data:/data
`,
			shouldFail: true,
			errorMsg:   "must be a relative path in demo mode",
		},
		{
			name:    "invalid relative path in demo mode",
			appName: "test-app",
			yamlContent: `
version: "3.8"
services:
  web:
    image: nginx:latest
    volumes:
      - ./random/path:/data
`,
			shouldFail: true,
			errorMsg:   "is not allowed",
		},
		{
			name:    "nested path not allowed",
			appName: "test-app",
			yamlContent: `
version: "3.8"
services:
  web:
    image: nginx:latest
    volumes:
      - ./apps/test-app/volumes/data:/usr/share/nginx/html
`,
			shouldFail: true,
			errorMsg:   "is not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator(tt.appName)
			err := validator.ValidateCompose([]byte(tt.yamlContent))

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail but it passed")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing \"%s\", got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass but got error: %v", err)
				}
			}
		})
	}
}
