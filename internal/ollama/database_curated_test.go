//go:build cgo
// +build cgo

package ollama

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	statements := []string{
		`CREATE TABLE ollama_models (
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
		)`,
		`CREATE TABLE ollama_download_jobs (
			id TEXT PRIMARY KEY,
			model_name TEXT NOT NULL,
			status TEXT DEFAULT 'queued',
			started_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (model_name) REFERENCES ollama_models(name)
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			db.Close() //nolint:errcheck,gosec // Test cleanup
			t.Fatalf("failed to prepare schema: %v", err)
		}
	}

	return db
}

func TestGetAllModelsMergesCuratedAndRecords(t *testing.T) {
	if len(CuratedModels) < 2 {
		t.Skip("insufficient curated models configured for test")
	}

	db := setupTestDatabase(t)
	defer db.Close() //nolint:errcheck,gosec // Test cleanup

	curated := CuratedModels[0]
	curatedCopy := curated
	if err := CreateModel(db, &curatedCopy); err != nil {
		t.Fatalf("failed to seed curated model: %v", err)
	}
	if err := UpdateModelStatus(db, curated.Name, StatusCompleted, 100); err != nil {
		t.Fatalf("failed to update curated model status: %v", err)
	}

	customModel := &OllamaModel{
		Name:         "custom/example:latest",
		DisplayName:  "Example Custom",
		Description:  "Custom model for testing",
		Category:     "custom",
		SizeEstimate: "Size varies",
		Status:       StatusCompleted,
		Progress:     100,
		UpdatedAt:    time.Now(),
	}
	if err := CreateModel(db, customModel); err != nil {
		t.Fatalf("failed to seed custom model: %v", err)
	}

	models, err := GetAllModels(db)
	if err != nil {
		t.Fatalf("GetAllModels failed: %v", err)
	}

	var (
		foundCurated       bool
		foundSecondCurated bool
		foundCustom        bool
	)

	secondCurated := CuratedModels[1]

	for _, model := range models {
		switch model.Name {
		case curated.Name:
			foundCurated = true
			if model.Status != StatusCompleted {
				t.Errorf("expected curated model status %q, got %q", StatusCompleted, model.Status)
			}
			if model.Progress != 100 {
				t.Errorf("expected curated model progress 100, got %d", model.Progress)
			}
		case secondCurated.Name:
			foundSecondCurated = true
			if model.Status != StatusNotDownloaded {
				t.Errorf("expected second curated model default status %q, got %q", StatusNotDownloaded, model.Status)
			}
		case customModel.Name:
			foundCustom = true
			if model.Category != "custom" {
				t.Errorf("expected custom model category 'custom', got %q", model.Category)
			}
		}
	}

	if !foundCurated {
		t.Errorf("expected curated model %q in results", curated.Name)
	}
	if !foundSecondCurated {
		t.Errorf("expected curated model %q in results", secondCurated.Name)
	}
	if !foundCustom {
		t.Errorf("expected custom model %q in results", customModel.Name)
	}
}

func TestCreateDownloadJobEnsuresCuratedRecord(t *testing.T) {
	if len(CuratedModels) == 0 {
		t.Skip("no curated models configured")
	}

	db := setupTestDatabase(t)
	defer db.Close() //nolint:errcheck,gosec // Test cleanup

	curated := CuratedModels[0]

	job, err := CreateDownloadJob(db, curated.Name)
	if err != nil {
		t.Fatalf("CreateDownloadJob returned error: %v", err)
	}
	if job.ModelName != curated.Name {
		t.Errorf("expected job model %q, got %q", curated.Name, job.ModelName)
	}

	record, err := GetModel(db, curated.Name)
	if err != nil {
		t.Fatalf("GetModel returned error: %v", err)
	}
	if record == nil {
		t.Fatalf("expected curated model record to exist after queuing job")
		return // Make linter happy - t.Fatalf exits but linter doesn't know
	}
	if record.Status != StatusQueued {
		t.Errorf("expected curated model status %q, got %q", StatusQueued, record.Status)
	}
}

func TestGetModelReturnsNilWhenNoRecordExists(t *testing.T) {
	db := setupTestDatabase(t)
	defer db.Close() //nolint:errcheck,gosec // Test cleanup

	record, err := GetModel(db, "non-existent-model")
	if err != nil {
		t.Fatalf("GetModel returned error: %v", err)
	}
	if record != nil {
		t.Fatalf("expected nil record for non-existent model, got %#v", record)
	}
}
