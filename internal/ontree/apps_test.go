package ontree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ontree-co/treeos/internal/config"
)

func TestAppInstallInjectsOpenWebUIAdminEnv(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	t.Setenv("TREEOS_RUN_MODE", "demo")
	t.Setenv("TREEOS_OPENWEBUI_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("TREEOS_OPENWEBUI_ADMIN_PASSWORD", "password123")
	t.Setenv("TREEOS_OPENWEBUI_ADMIN_NAME", "TreeOS Admin")

	cfg := &config.Config{
		AppsDir:      filepath.Join(tmpDir, "apps"),
		DatabasePath: filepath.Join(tmpDir, "ontree.db"),
	}
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	done := manager.AppInstall(t.Context(), "openwebui", "", "")
	for range done {
	}

	envPath := filepath.Join(cfg.AppsDir, "openwebui", ".env")
	raw, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", envPath, err)
	}
	envContent := string(raw)

	required := []string{
		"WEBUI_ADMIN_EMAIL=admin@example.com",
		"WEBUI_ADMIN_PASSWORD=password123",
		"WEBUI_ADMIN_NAME=TreeOS Admin",
	}
	for _, line := range required {
		if !strings.Contains(envContent, line) {
			t.Fatalf("expected env to contain %q", line)
		}
	}
}
