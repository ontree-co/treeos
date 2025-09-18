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

	// Close any existing connection first
	if db != nil {
		db.Close()
		db = nil
	}

	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for better concurrency handling
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Enable WAL mode for better concurrency (if not already enabled)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("Warning: Could not enable WAL mode: %v", err)
	}

	// Set synchronous to NORMAL for better performance while maintaining safety
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		log.Printf("Warning: Could not set synchronous mode: %v", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations - this must complete synchronously
	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Force a checkpoint to ensure all changes are written
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("Warning: Could not checkpoint after migrations: %v", err)
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
			tailscale_auth_key TEXT,
			tailscale_tags TEXT DEFAULT 'tag:ontree-apps',
			agent_enabled INTEGER DEFAULT 0,
			agent_check_interval TEXT DEFAULT '5m',
			agent_llm_api_key TEXT,
			agent_llm_api_url TEXT,
			agent_llm_model TEXT,
			uptime_kuma_base_url TEXT
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
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			message TEXT NOT NULL,
			sender_type TEXT NOT NULL CHECK (sender_type IN ('user', 'agent', 'system')),
			sender_name TEXT NOT NULL,
			agent_model TEXT,
			agent_provider TEXT,
			status_level TEXT CHECK (status_level IN ('info', 'warning', 'error', 'critical') OR status_level IS NULL),
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_system_vital_logs_timestamp ON system_vital_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operations_status_created ON docker_operations(status, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operations_app_created ON docker_operations(app_name, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_operation_logs_operation_timestamp ON docker_operation_logs(operation_id, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_app_timestamp ON chat_messages(app_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_sender_type ON chat_messages(sender_type, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_app_sender ON chat_messages(app_id, sender_type)`,
		`CREATE TABLE IF NOT EXISTS ollama_models (
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
		`CREATE TABLE IF NOT EXISTS ollama_download_jobs (
			id TEXT PRIMARY KEY,
			model_name TEXT NOT NULL,
			status TEXT DEFAULT 'queued',
			started_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (model_name) REFERENCES ollama_models(name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ollama_models_status ON ollama_models(status)`,
		`CREATE INDEX IF NOT EXISTS idx_download_jobs_status ON ollama_download_jobs(status, created_at)`,
		`CREATE TABLE IF NOT EXISTS update_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version TEXT NOT NULL,
			channel TEXT NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT,
			started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_update_history_started_at ON update_history(started_at DESC)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	// These ALTER statements are for migrating older databases that don't have these columns
	// They're no longer needed since the CREATE TABLE statements already include them
	// We'll check if migration is needed by checking for a column that was added later
	if err := migrateColumnsIfNeeded(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// migrateColumnsIfNeeded checks if columns exist before trying to add them
func migrateColumnsIfNeeded() error {
	// Don't skip migrations based on a single column - run each migration check individually
	// This allows us to add new migrations later without issues

	// Migration queries for older databases
	migrations := []struct {
		table  string
		column string
		query  string
	}{
		{"system_setup", "public_base_domain", `ALTER TABLE system_setup ADD COLUMN public_base_domain TEXT`},
		{"system_setup", "tailscale_auth_key", `ALTER TABLE system_setup ADD COLUMN tailscale_auth_key TEXT`},
		{"system_setup", "tailscale_tags", `ALTER TABLE system_setup ADD COLUMN tailscale_tags TEXT DEFAULT 'tag:ontree-apps'`},
		{"system_setup", "agent_enabled", `ALTER TABLE system_setup ADD COLUMN agent_enabled INTEGER DEFAULT 0`},
		{"system_setup", "agent_check_interval", `ALTER TABLE system_setup ADD COLUMN agent_check_interval TEXT DEFAULT '5m'`},
		{"system_setup", "agent_llm_api_key", `ALTER TABLE system_setup ADD COLUMN agent_llm_api_key TEXT`},
		{"system_setup", "agent_llm_api_url", `ALTER TABLE system_setup ADD COLUMN agent_llm_api_url TEXT`},
		{"system_setup", "agent_llm_model", `ALTER TABLE system_setup ADD COLUMN agent_llm_model TEXT`},
		{"system_setup", "uptime_kuma_base_url", `ALTER TABLE system_setup ADD COLUMN uptime_kuma_base_url TEXT`},
		{"system_setup", "update_channel", `ALTER TABLE system_setup ADD COLUMN update_channel TEXT DEFAULT 'beta'`},
		{"system_vital_logs", "upload_rate", `ALTER TABLE system_vital_logs ADD COLUMN upload_rate INTEGER DEFAULT 0`},
		{"system_vital_logs", "download_rate", `ALTER TABLE system_vital_logs ADD COLUMN download_rate INTEGER DEFAULT 0`},
		{"system_vital_logs", "gpu_load", `ALTER TABLE system_vital_logs ADD COLUMN gpu_load REAL DEFAULT 0`},
		{"system_setup", "node_icon", `ALTER TABLE system_setup ADD COLUMN node_icon TEXT DEFAULT 'tree1.png'`},
	}

	for _, m := range migrations {
		// Check if column exists
		var colCount int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'`, m.table, m.column)
		row := db.QueryRow(query)
		if err := row.Scan(&colCount); err != nil {
			return fmt.Errorf("failed to check column %s.%s: %w", m.table, m.column, err)
		}

		// Only add column if it doesn't exist
		if colCount == 0 {
			if _, err := db.Exec(m.query); err != nil {
				return fmt.Errorf("failed to add column %s.%s: %w", m.table, m.column, err)
			}
			log.Printf("Added column %s.%s", m.table, m.column)
		}
	}

	return nil
}
