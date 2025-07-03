package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func GetDB() *sql.DB {
	return db
}

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

	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Printf("Database initialized successfully at %s", dbPath)
	return nil
}

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
			node_description TEXT
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
		`CREATE INDEX IF NOT EXISTS idx_system_vital_logs_timestamp ON system_vital_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operations_status_created ON docker_operations(status, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operations_app_created ON docker_operations(app_name, created_at)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}