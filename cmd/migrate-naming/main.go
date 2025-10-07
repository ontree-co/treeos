// Package main provides a CLI tool to migrate Docker containers to the new naming scheme.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"treeos/internal/config"
	"treeos/internal/naming"
)

func main() {
	var appsDir string
	var dryRun bool
	flag.StringVar(&appsDir, "apps-dir", config.GetAppsPath(), "Path to the apps directory")
	flag.BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	flag.Parse()

	fmt.Printf("=== Docker Container Naming Migration Tool ===\n")
	fmt.Printf("Apps directory: %s\n", appsDir)
	if dryRun {
		fmt.Printf("Mode: DRY RUN (no changes will be made)\n")
	} else {
		fmt.Printf("Mode: APPLY CHANGES\n")
	}
	fmt.Printf("\n")

	// Check if directory exists
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		log.Fatalf("Apps directory does not exist: %s", appsDir)
	}

	// Read all subdirectories
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		log.Fatalf("Failed to read apps directory: %v", err)
	}

	migrated := 0
	skipped := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip the database file
		if entry.Name() == "ontree.db" {
			continue
		}

		appPath := filepath.Join(appsDir, entry.Name())

		// Check if docker-compose.yml exists
		composePath := filepath.Join(appPath, "docker-compose.yml")
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			fmt.Printf("⏭️  Skipping %s: no docker-compose.yml\n", entry.Name())
			skipped++
			continue
		}

		// Check if .env already exists
		envPath := filepath.Join(appPath, ".env")
		hasEnv := false
		if _, err := os.Stat(envPath); err == nil {
			// Check if it already has COMPOSE_PROJECT_NAME
			content, err := os.ReadFile(envPath) //nolint:gosec // File path from trusted directory listing
			if err == nil && strings.Contains(string(content), "COMPOSE_PROJECT_NAME=") {
				fmt.Printf("✓  %s: already has .env with COMPOSE_PROJECT_NAME\n", entry.Name())
				skipped++
				continue
			}
			hasEnv = true
		}

		// Generate the .env content
		appIdentifier := naming.GetAppIdentifier(appPath)
		projectName := naming.GetComposeProjectName(appIdentifier)

		fmt.Printf("📦 %s:\n", entry.Name())
		fmt.Printf("   App Identifier: %s\n", appIdentifier)
		fmt.Printf("   Project Name: %s\n", projectName)
		fmt.Printf("   Container Pattern: %s-*\n", projectName)

		if !dryRun {
			// Create or update .env file
			err := naming.GenerateEnvFile(appPath)
			if err != nil {
				fmt.Printf("   ❌ Failed to create .env: %v\n", err)
				continue
			}

			if hasEnv {
				fmt.Printf("   ✅ Updated existing .env file\n")
			} else {
				fmt.Printf("   ✅ Created new .env file\n")
			}
			migrated++
		} else {
			if hasEnv {
				fmt.Printf("   → Would update existing .env file\n")
			} else {
				fmt.Printf("   → Would create new .env file\n")
			}
			migrated++
		}

		fmt.Printf("\n")
	}

	fmt.Printf("=== Summary ===\n")
	if dryRun {
		fmt.Printf("Would migrate: %d apps\n", migrated)
	} else {
		fmt.Printf("Migrated: %d apps\n", migrated)
	}
	fmt.Printf("Skipped: %d apps\n", skipped)

	if !dryRun && migrated > 0 {
		fmt.Printf("\n⚠️  IMPORTANT: You need to recreate containers for the naming to take effect:\n")
		fmt.Printf("   1. Stop existing containers: docker compose down\n")
		fmt.Printf("   2. Start with new names: docker compose up -d\n")
		fmt.Printf("\n")
		fmt.Printf("   Or use the OnTree UI to stop and start each app.\n")
	}
}
