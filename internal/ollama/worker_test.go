//go:build cgo
// +build cgo

package ollama

import (
	"database/sql"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestCancelDownloadKillsContainerProcess tests that cancelling a download
// properly kills the process inside the container, not just the container exec process
func TestCancelDownloadKillsContainerProcess(t *testing.T) {
	// Skip if not in CI or if no Docker available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping test")
	}

	// Check if test container exists
	checkCmd := exec.Command("docker", "ps", "--filter", "label=ontree.inference=true", "--format", "{{.Names}}")
	output, err := checkCmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		t.Skip("No Ollama container with ontree.inference=true label found, skipping test")
	}
	containerName := strings.Split(strings.TrimSpace(string(output)), "\n")[0]

	// Create in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create the necessary tables
	_, err = db.Exec(`
		CREATE TABLE ollama_models (
			name TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			size_estimate TEXT,
			description TEXT,
			category TEXT,
			status TEXT DEFAULT 'not_downloaded',
			progress INTEGER DEFAULT 0,
			last_error TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create worker
	w := NewWorker(db, containerName)
	w.Start(1)
	defer w.Stop()

	// Start a fake long-running process in the container to simulate ollama pull
	// We use a model name that doesn't exist to ensure it runs for a while
	testModel := "test-model-that-does-not-exist:latest"

	// Add a job to download a model
	job := DownloadJob{
		ID:        "test-job-1",
		ModelName: testModel,
	}
	w.AddJob(job)

	// Give it a moment to start
	time.Sleep(2 * time.Second)

	// Check if there's a process running in the container (for debugging)
	psCmd := exec.Command("docker", "exec", containerName, "sh", "-c",
		"ps aux | grep 'ollama pull' | grep -v grep")
	psOutput, _ := psCmd.Output()
	if initialProcesses := strings.TrimSpace(string(psOutput)); initialProcesses != "" {
		t.Logf("Initial ollama processes found: %s", initialProcesses)
	}

	// Cancel the download
	err = w.CancelDownload(testModel)
	if err != nil && !strings.Contains(err.Error(), "no active download") {
		t.Errorf("Failed to cancel download: %v", err)
	}

	// Wait for cancellation to take effect
	time.Sleep(2 * time.Second)

	// Check if the ollama pull process is still running in the container
	psCmd2 := exec.Command("docker", "exec", containerName, "sh", "-c",
		"ps aux | grep 'ollama pull' | grep -v grep")
	psOutput2, _ := psCmd2.Output()
	remainingProcesses := strings.TrimSpace(string(psOutput2))

	if remainingProcesses != "" && strings.Contains(remainingProcesses, testModel) {
		t.Errorf("Ollama pull process still running in container after cancellation:\n%s", remainingProcesses)
	}

	// Also verify no container exec processes remain
	hostPsCmd := exec.Command("sh", "-c", "ps aux | grep 'docker exec' | grep 'ollama pull' | grep -v grep")
	hostOutput, _ := hostPsCmd.Output()
	if strings.TrimSpace(string(hostOutput)) != "" {
		t.Errorf("Container exec process still running on host after cancellation:\n%s", hostOutput)
	}
}

// TestCancelDownloadCleansUpPartialDownload tests that cancelling removes partial downloads
func TestCancelDownloadCleansUpPartialDownload(t *testing.T) {
	// This would test that after cancellation:
	// 1. The partial model is removed from Ollama's storage
	// 2. The database is updated correctly
	// 3. The proper status update is sent

	t.Skip("Implement this test with a mock or test container")
}

// TestCancelNonExistentDownload tests error handling for cancelling non-existent downloads
func TestCancelNonExistentDownload(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	w := NewWorker(db, "test-container")

	err = w.CancelDownload("non-existent-model")
	if err == nil {
		t.Error("Expected error when cancelling non-existent download, got nil")
	}
	if !strings.Contains(err.Error(), "no active download found") {
		t.Errorf("Expected 'no active download found' error, got: %v", err)
	}
}
