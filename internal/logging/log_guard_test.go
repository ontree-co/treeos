package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoStdlibLogUsage guards against reintroducing direct uses of the stdlib log package
// outside of internal/logging. All log calls should go through this package so they respect
// the configured log level and writers.
func TestNoStdlibLogUsage(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		// Allow the logger implementation itself to import log.
		if filepath.Base(path) == "logger.go" && strings.Contains(path, string(filepath.Separator)+"logging"+string(filepath.Separator)) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), "\n\t\"log\"") || strings.Contains(string(content), "import \"log\"") {
			t.Errorf("stdlib log import found in %s; use treeos/internal/logging instead", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
}
