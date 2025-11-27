package main

import (
	"os"
	"testing"
)

func TestNormalizeListenAddr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "bare port number",
			input:   "3000",
			want:    ":3000",
			wantErr: false,
		},
		{
			name:    "port with colon prefix",
			input:   ":3000",
			want:    ":3000",
			wantErr: false,
		},
		{
			name:    "full address with host",
			input:   "127.0.0.1:3000",
			want:    "127.0.0.1:3000",
			wantErr: false,
		},
		{
			name:    "IPv6 address",
			input:   "[::1]:3000",
			want:    "[::1]:3000",
			wantErr: false,
		},
		{
			name:    "all interfaces",
			input:   "0.0.0.0:3000",
			want:    "0.0.0.0:3000",
			wantErr: false,
		},
		{
			name:    "invalid port - too high",
			input:   "70000",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid port - zero",
			input:   "0",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid port - negative",
			input:   "-1",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid port - not a number",
			input:   "abc",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeListenAddr(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeListenAddr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("normalizeListenAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{
			name:    "valid port 80",
			port:    "80",
			wantErr: false,
		},
		{
			name:    "valid port 3000",
			port:    "3000",
			wantErr: false,
		},
		{
			name:    "valid port 65535",
			port:    "65535",
			wantErr: false,
		},
		{
			name:    "invalid port 0",
			port:    "0",
			wantErr: true,
		},
		{
			name:    "invalid port 65536",
			port:    "65536",
			wantErr: true,
		},
		{
			name:    "invalid port negative",
			port:    "-1",
			wantErr: true,
		},
		{
			name:    "invalid port string",
			port:    "http",
			wantErr: true,
		},
		{
			name:    "empty port",
			port:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validatePort(tt.port); (err != nil) != tt.wantErr {
				t.Errorf("validatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetupDemoDirs(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	appsDir := tmpDir + "/apps"

	// Save current directory and change to temp dir
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck // Test cleanup

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test creating demo directories
	err = setupDemoDirs(appsDir)
	if err != nil {
		t.Fatalf("setupDemoDirs() error = %v", err)
	}

	// Verify directories were created
	dirs := []string{
		appsDir,
		"./shared/ollama",
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

// Note: Testing setupLinuxDirs and setupDarwinDirs requires root privileges
// and system-specific behavior, so they are better tested through integration tests
