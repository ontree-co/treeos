package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrationCompletion verifies that all migrations complete successfully
// and that the verification can detect incomplete migrations
func TestMigrationCompletion(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize the database
	err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer Close()

	// Verify critical columns exist
	criticalChecks := []struct {
		table  string
		column string
	}{
		{"system_setup", "update_channel"},
		{"update_history", "channel"},
		{"system_vital_logs", "gpu_load"},
	}

	db := GetDB()
	for _, check := range criticalChecks {
		var colCount int
		query := `SELECT COUNT(*) FROM pragma_table_info('` + check.table + `') WHERE name='` + check.column + `'`
		row := db.QueryRow(query)
		if err := row.Scan(&colCount); err != nil {
			t.Errorf("Failed to verify column %s.%s: %v", check.table, check.column, err)
			continue
		}
		if colCount == 0 {
			t.Errorf("Migration incomplete: column %s.%s does not exist", check.table, check.column)
		}
	}

	// Test that we can actually read from the new columns
	var testValue sql.NullString
	err = db.QueryRow("SELECT update_channel FROM system_setup LIMIT 1").Scan(&testValue)
	if err != nil && err != sql.ErrNoRows {
		t.Errorf("Cannot read from update_channel column: %v", err)
	}

	// Test WAL checkpoint
	_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		t.Logf("Warning: Could not checkpoint WAL: %v", err)
	}
}

// TestMigrationIdempotency verifies that running migrations multiple times is safe
func TestMigrationIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize once
	err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("First initialization failed: %v", err)
	}
	Close()

	// Initialize again - should not fail
	err = Initialize(dbPath)
	if err != nil {
		t.Fatalf("Second initialization failed: %v", err)
	}
	defer Close()

	// Verify database is still functional
	db := GetDB()
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM system_setup").Scan(&count)
	if err != nil {
		t.Errorf("Database not functional after re-initialization: %v", err)
	}
}

// TestMigrationAfterRestart simulates what happens after a self-update
func TestMigrationAfterRestart(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create initial database without the new columns
	// This simulates an old version of the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create basic tables without new columns
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS system_setup (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		is_setup_complete INTEGER DEFAULT 0,
		node_name TEXT DEFAULT 'OnTree Node'
	)`)
	if err != nil {
		t.Fatalf("Failed to create basic table: %v", err)
	}
	db.Close()

	// Now initialize with our current code - should add missing columns
	err = Initialize(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize after simulated restart: %v", err)
	}
	defer Close()

	// Verify the new column was added
	currentDB := GetDB()
	var colCount int
	query := `SELECT COUNT(*) FROM pragma_table_info('system_setup') WHERE name='update_channel'`
	row := currentDB.QueryRow(query)
	if err := row.Scan(&colCount); err != nil {
		t.Errorf("Failed to check for update_channel column: %v", err)
	}
	if colCount == 0 {
		t.Errorf("Migration did not add update_channel column")
	}

	// Verify we can read from it
	var testValue sql.NullString
	err = currentDB.QueryRow("SELECT update_channel FROM system_setup LIMIT 1").Scan(&testValue)
	if err != nil && err != sql.ErrNoRows {
		t.Errorf("Cannot read from newly added update_channel column: %v", err)
	}
}