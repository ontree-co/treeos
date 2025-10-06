// Package migration contains migration tools for OnTree application data and configurations.
package migration

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"treeos/internal/config"
	"treeos/internal/database"
	"treeos/internal/yamlutil"
)

// DeployedApp represents the old deployed_apps table structure
// This is kept temporarily for migration purposes only
type DeployedApp struct {
	ID            string
	Name          string
	DockerCompose string
	Subdomain     string
	HostPort      int
	IsExposed     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// MigrateDeployedAppsToCompose migrates all app metadata from the deployed_apps table
// to the x-ontree section in docker-compose.yml files
func MigrateDeployedAppsToCompose(cfg *config.Config) error {
	log.Println("Starting migration of deployed_apps to docker-compose.yml files...")

	// Create backup directory
	backupDir := filepath.Join(cfg.AppsDir, ".backup-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	log.Printf("Created backup directory: %s", backupDir)

	// Get all deployed apps from database
	apps, err := getAllDeployedApps()
	if err != nil {
		return fmt.Errorf("failed to read deployed_apps: %w", err)
	}
	log.Printf("Found %d apps to migrate", len(apps))

	if len(apps) == 0 {
		log.Println("No apps found in deployed_apps table. Migration complete.")
		return nil
	}

	// Process each app
	successCount := 0
	for _, app := range apps {
		log.Printf("Migrating app: %s", app.Name)

		appPath := filepath.Join(cfg.AppsDir, app.Name)
		composePath := filepath.Join(appPath, "docker-compose.yml")

		// Check if app directory exists
		if _, err := os.Stat(appPath); os.IsNotExist(err) {
			log.Printf("WARNING: App directory not found for %s, skipping", app.Name)
			continue
		}

		// Check if compose file exists
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			log.Printf("WARNING: docker-compose.yml not found for %s, skipping", app.Name)
			continue
		}

		// Backup the original compose file
		backupPath := filepath.Join(backupDir, app.Name+"-docker-compose.yml")
		if err := copyFile(composePath, backupPath); err != nil {
			log.Printf("ERROR: Failed to backup compose file for %s: %v", app.Name, err)
			continue
		}

		// Read the current compose file
		compose, err := yamlutil.ReadComposeWithMetadata(composePath)
		if err != nil {
			log.Printf("ERROR: Failed to read compose file for %s: %v", app.Name, err)
			continue
		}

		// Create metadata from database record
		metadata := &yamlutil.OnTreeMetadata{
			Subdomain: app.Subdomain,
			HostPort:  app.HostPort,
			IsExposed: app.IsExposed,
		}

		// Set the metadata in the compose file
		yamlutil.SetOnTreeMetadata(compose, metadata)

		// Write the updated compose file
		if err := yamlutil.WriteComposeWithMetadata(composePath, compose); err != nil {
			log.Printf("ERROR: Failed to write compose file for %s: %v", app.Name, err)
			continue
		}

		log.Printf("✓ Successfully migrated app: %s (subdomain: %s, port: %d, exposed: %v)",
			app.Name, app.Subdomain, app.HostPort, app.IsExposed)
		successCount++
	}

	log.Printf("Migration complete: %d/%d apps migrated successfully", successCount, len(apps))

	if successCount < len(apps) {
		log.Printf("WARNING: %d apps failed to migrate. Check logs above for details.", len(apps)-successCount)
		log.Printf("Backup files are available in: %s", backupDir)
		return fmt.Errorf("migration completed with errors: %d/%d apps migrated", successCount, len(apps))
	}

	log.Println("All apps migrated successfully!")
	log.Printf("Backup files are available in: %s", backupDir)
	log.Println("You can now safely remove the deployed_apps table from the database.")

	return nil
}

// getAllDeployedApps retrieves all records from the deployed_apps table
func getAllDeployedApps() ([]DeployedApp, error) {
	db := database.GetDB()

	query := `
		SELECT id, name, docker_compose, subdomain, host_port, is_exposed, created_at, updated_at
		FROM deployed_apps
		ORDER BY name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query deployed_apps: %w", err)
	}
	defer rows.Close() //nolint:errcheck // Cleanup, error not critical

	var apps []DeployedApp
	for rows.Next() {
		var app DeployedApp
		err := rows.Scan(
			&app.ID,
			&app.Name,
			&app.DockerCompose,
			&app.Subdomain,
			&app.HostPort,
			&app.IsExposed,
			&app.CreatedAt,
			&app.UpdatedAt,
		)
		if err != nil {
			log.Printf("WARNING: Failed to scan row: %v", err)
			continue
		}
		apps = append(apps, app)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return apps, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src) //nolint:gosec // File path from trusted migration source
	if err != nil {
		return err
	}

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	// Use 0644 for backup files as they may need to be accessible for recovery
	return os.WriteFile(dst, input, 0644) // #nosec G306 - backup files should be readable
}
