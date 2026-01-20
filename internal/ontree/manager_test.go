package ontree

import (
	"path/filepath"
	"testing"

	"github.com/ontree-co/treeos/internal/config"
)

func TestSetupInitAndStatus(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		AppsDir:      filepath.Join(tmpDir, "apps"),
		DatabasePath: filepath.Join(tmpDir, "ontree.db"),
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	status, err := manager.SetupStatus(t.Context())
	if err != nil {
		t.Fatalf("SetupStatus() error = %v", err)
	}
	if status.Complete {
		t.Fatalf("expected setup incomplete, got complete")
	}

	err = manager.SetupInit(t.Context(), "admin", "password123", "Test Node", "logo.png")
	if err != nil {
		t.Fatalf("SetupInit() error = %v", err)
	}

	status, err = manager.SetupStatus(t.Context())
	if err != nil {
		t.Fatalf("SetupStatus() error = %v", err)
	}
	if !status.Complete {
		t.Fatalf("expected setup complete, got incomplete")
	}
	if status.NodeName != "Test Node" {
		t.Fatalf("expected node name Test Node, got %s", status.NodeName)
	}
	if status.NodeIcon != "logo.png" {
		t.Fatalf("expected node icon logo.png, got %s", status.NodeIcon)
	}
}
