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
	defer Close() //nolint:errcheck // Test cleanup

	// Verify critical columns exist
	criticalChecks := []struct {
		table  string
		column string
	}{
		{"system_setup", "update_channel"},
		{"system_setup", "node_icon"},
		{"update_history", "channel"},
		{"system_vital_logs", "gpu_load"},
	}

	db := GetDB()
	for _, check := range criticalChecks {
		var colCount int
		//nolint:gosec // Test SQL with trusted schema check values
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
	Close() //nolint:errcheck,gosec // Test cleanup

	// Initialize again - should not fail
	err = Initialize(dbPath)
	if err != nil {
		t.Fatalf("Second initialization failed: %v", err)
	}
	defer Close() //nolint:errcheck // Test cleanup

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
	db.Close() //nolint:errcheck,gosec // Test cleanup

	// Now initialize with our current code - should add missing columns
	err = Initialize(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize after simulated restart: %v", err)
	}
	defer Close() //nolint:errcheck // Test cleanup

	// Verify the new columns were added
	currentDB := GetDB()
	columnsToCheck := []string{"update_channel", "node_icon"}

	for _, col := range columnsToCheck {
		var colCount int
		//nolint:gosec // Test SQL with trusted column names
		query := `SELECT COUNT(*) FROM pragma_table_info('system_setup') WHERE name='` + col + `'`
		row := currentDB.QueryRow(query)
		if err := row.Scan(&colCount); err != nil {
			t.Errorf("Failed to check for %s column: %v", col, err)
		}
		if colCount == 0 {
			t.Errorf("Migration did not add %s column", col)
		}
	}

	// Verify we can read from the new columns
	var testValue sql.NullString
	err = currentDB.QueryRow("SELECT update_channel FROM system_setup LIMIT 1").Scan(&testValue)
	if err != nil && err != sql.ErrNoRows {
		t.Errorf("Cannot read from newly added update_channel column: %v", err)
	}

	// Test node_icon column specifically
	var nodeIcon sql.NullString
	err = currentDB.QueryRow("SELECT node_icon FROM system_setup LIMIT 1").Scan(&nodeIcon)
	if err != nil && err != sql.ErrNoRows {
		t.Errorf("Cannot read from newly added node_icon column: %v", err)
	}
}
