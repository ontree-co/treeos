// Package main provides a CLI tool to migrate the TreeOS database schema.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"strings"
	"treeos/internal/logging"

	_ "github.com/mattn/go-sqlite3"
	"treeos/internal/config"
)

func main() {
	var dbPath string
	var dryRun bool
	flag.StringVar(&dbPath, "db", config.GetDatabasePath(), "Path to the database")
	flag.BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	flag.Parse()

	fmt.Printf("=== Database App ID Migration Tool ===\n")
	fmt.Printf("Database: %s\n", dbPath)
	if dryRun {
		fmt.Printf("Mode: DRY RUN (no changes will be made)\n")
	} else {
		fmt.Printf("Mode: APPLY CHANGES\n")
	}
	fmt.Printf("\n")

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close() //nolint:errcheck // Cleanup, error not critical

	// Check chat_messages table
	fmt.Printf("Checking chat_messages table...\n")

	rows, err := db.Query("SELECT DISTINCT app_id FROM chat_messages")
	if err != nil {
		logging.Fatalf("Failed to query chat_messages: %v", err)
	}

	var appIDs []string
	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			continue
		}
		appIDs = append(appIDs, appID)
	}
	rows.Close() //nolint:errcheck,gosec // Cleanup, error not critical

	migrated := 0
	for _, oldID := range appIDs {
		// Check if it has the old format (app-prefix)
		if strings.HasPrefix(oldID, "app-") {
			newID := strings.TrimPrefix(oldID, "app-")
			newID = strings.ToLower(newID)

			fmt.Printf("  %s → %s\n", oldID, newID)

			if !dryRun {
				// Update the app_id
				_, err := db.Exec("UPDATE chat_messages SET app_id = ? WHERE app_id = ?", newID, oldID)
				if err != nil {
					fmt.Printf("    ❌ Failed to update: %v\n", err)
				} else {
					fmt.Printf("    ✅ Updated\n")
					migrated++
				}
			} else {
				migrated++
			}
		} else {
			fmt.Printf("  %s (already in new format)\n", oldID)
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	if dryRun {
		fmt.Printf("Would migrate: %d app IDs\n", migrated)
	} else {
		fmt.Printf("Migrated: %d app IDs\n", migrated)
	}
}
