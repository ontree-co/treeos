package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeProjectName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"My App", "my-app"},
		{"My_App", "my-app"},
		{"   Example!  ", "example"},
		{"123Project", "123project"},
		{"🚀Rocket", "rocket"},
		{"already-good", "already-good"},
	}

	for _, tc := range cases {
		got := sanitizeProjectName(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeProjectName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestProjectNameFromEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# sample file\nCOMPOSE_PROJECT_NAME=my-compose\nOTHER=value\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("failed to write env file: %v", err)
	}

	got := projectNameFromEnv(dir)
	if got != "my-compose" {
		t.Fatalf("expected project name 'my-compose', got %q", got)
	}
}

func TestResolveProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("version: '3'\nservices:{}"), 0o644); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("failed to write compose file: %v", err)
	}

	opts := Options{WorkingDir: dir}
	_, project, err := resolveProject(opts)
	if err != nil {
		t.Fatalf("resolveProject returned error: %v", err)
	}
	expect := sanitizeProjectName(filepath.Base(dir))
	if project != expect {
		t.Fatalf("expected project %q, got %q", expect, project)
	}
}

func TestSortContainerSummaries(t *testing.T) {
	containers := []ContainerSummary{
		{Name: "b", Service: "svc2"},
		{Name: "a", Service: "svc2"},
		{Name: "a", Service: "svc1"},
	}

	sortContainerSummaries(containers)

	if containers[0].Name != "a" || containers[0].Service != "svc1" {
		t.Fatalf("unexpected order after sort: %+v", containers)
	}
	if containers[1].Name != "a" || containers[1].Service != "svc2" {
		t.Fatalf("unexpected order after sort: %+v", containers)
	}
	if containers[2].Name != "b" {
		t.Fatalf("unexpected order after sort: %+v", containers)
	}
}
