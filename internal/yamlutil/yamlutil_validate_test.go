package yamlutil

import (
	"strings"
	"testing"
)

// TestValidateComposeFile tests the YAML validation function
func TestValidateComposeFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid docker-compose.yml",
			content: `version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"`,
			wantErr: false,
		},
		{
			name: "Valid with build instead of image",
			content: `version: '3.8'
services:
  app:
    build: .
    ports:
      - "3000:3000"`,
			wantErr: false,
		},
		{
			name: "Missing version",
			content: `services:
  web:
    image: nginx:alpine`,
			wantErr: true,
			errMsg:  "missing 'version' field",
		},
		{
			name:    "Empty version",
			content: `version: ""
services:
  web:
    image: nginx:alpine`,
			wantErr: true,
			errMsg:  "missing 'version' field",
		},
		{
			name:    "Missing services",
			content: `version: '3.8'`,
			wantErr: true,
			errMsg:  "missing 'services' section",
		},
		{
			name:    "Empty services",
			content: `version: '3.8'
services: {}`,
			wantErr: true,
			errMsg:  "missing 'services' section",
		},
		{
			name:    "Service without image or build",
			content: `version: '3.8'
services:
  web:
    ports:
      - "8080:80"`,
			wantErr: true,
			errMsg:  "must have either 'image' or 'build' field",
		},
		{
			name:    "Invalid YAML syntax",
			content: `version: '3.8'
services:
  web
    image: nginx`,
			wantErr: true,
			errMsg:  "invalid YAML syntax",
		},
		{
			name:    "Invalid service structure",
			content: `version: '3.8'
services:
  web: "not a map"`,
			wantErr: true,
			errMsg:  "invalid structure",
		},
		{
			name: "Multiple services (all valid)",
			content: `version: '3.8'
services:
  web:
    image: nginx:alpine
  db:
    image: postgres:13`,
			wantErr: false,
		},
		{
			name: "With x-ontree metadata",
			content: `version: '3.8'
services:
  app:
    image: myapp:latest
x-ontree:
  subdomain: myapp
  host_port: 8080
  is_exposed: true`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComposeFile(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateComposeFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateComposeFile() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

// TestValidateComposeFileEdgeCases tests edge cases for YAML validation
func TestValidateComposeFileEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Completely empty file",
			content: "",
			wantErr: true,
			errMsg:  "missing 'version' field", // Empty YAML is valid but missing version
		},
		{
			name:    "Just whitespace",
			content: "   \n\t\n   ",
			wantErr: true,
			errMsg:  "invalid YAML syntax", // Tabs are invalid in YAML
		},
		{
			name:    "Malformed YAML with tabs",
			content: "version: '3.8'\nservices:\n\tweb:\n\t\timage: nginx",
			wantErr: true,
			errMsg:  "invalid YAML syntax",
		},
		{
			name: "Service with both image and build",
			content: `version: '3.8'
services:
  app:
    image: myapp:latest
    build: .`,
			wantErr: false, // This is actually valid in docker-compose
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComposeFile(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateComposeFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateComposeFile() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}