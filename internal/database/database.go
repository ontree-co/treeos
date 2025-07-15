// Package database provides database connectivity and management for the OnTree application.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

var db *sql.DB

// GetDB returns the current database connection.
func GetDB() *sql.DB {
	return db
}

// Initialize opens a connection to the SQLite database and runs migrations.
func Initialize(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// For now, keep the old createTables() for backward compatibility
	// Once migrations are fully tested, this can be removed
	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Printf("Database initialized successfully at %s", dbPath)
	return nil
}

// New creates and initializes a new database connection
func New(dbPath string) (*sql.DB, error) {
	if err := Initialize(dbPath); err != nil {
		return nil, err
	}
	return db, nil
}

// Close closes the database connection.
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			email TEXT,
			first_name TEXT,
			last_name TEXT,
			is_staff INTEGER DEFAULT 0,
			is_superuser INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			date_joined DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS system_setup (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			is_setup_complete INTEGER DEFAULT 0,
			setup_date DATETIME,
			node_name TEXT DEFAULT 'OnTree Node',
			node_description TEXT,
			public_base_domain TEXT,
			tailscale_base_domain TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS system_vital_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			cpu_percent REAL NOT NULL,
			memory_percent REAL NOT NULL,
			disk_usage_percent REAL NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS docker_operations (
			id TEXT PRIMARY KEY,
			operation_type TEXT NOT NULL,
			app_name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			progress INTEGER DEFAULT 0,
			progress_message TEXT,
			error_message TEXT,
			metadata TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS docker_operation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_id TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			details TEXT,
			FOREIGN KEY (operation_id) REFERENCES docker_operations(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_system_vital_logs_timestamp ON system_vital_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operations_status_created ON docker_operations(status, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operations_app_created ON docker_operations(app_name, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operation_logs_operation_timestamp ON docker_operation_logs(operation_id, timestamp)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	// Add domain columns to existing system_setup table (safe to run multiple times)
	alterQueries := []string{
		`ALTER TABLE system_setup ADD COLUMN public_base_domain TEXT`,
		`ALTER TABLE system_setup ADD COLUMN tailscale_base_domain TEXT`,
		`ALTER TABLE system_vital_logs ADD COLUMN network_rx_bytes INTEGER DEFAULT 0`,
		`ALTER TABLE system_vital_logs ADD COLUMN network_tx_bytes INTEGER DEFAULT 0`,
	}

	for _, query := range alterQueries {
		// Ignore errors as columns may already exist
		_, err := db.Exec(query)
		if err != nil {
			// This is expected if the column already exists, which is fine
			log.Printf("Migration query (expected to fail if column exists): %v", err)
		}
	}

	return nil
}
