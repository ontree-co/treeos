// Package main is the entry point for the OnTree server application
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/joho/godotenv"
	"treeos/internal/config"
	"treeos/internal/database"
	"treeos/internal/logging"
	"treeos/internal/migration"
	"treeos/internal/server"
	"treeos/internal/telemetry"
	"treeos/internal/version"
)

func main() {
	// Load .env file if it exists (for development)
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, especially in production
		// Only log in debug mode
		if os.Getenv("DEBUG") == "true" {
			log.Printf("No .env file found or error loading it: %v", err)
		}
	}

	// Handle version flag first, before loading configuration
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version" || os.Args[1] == "version") {
		versionInfo := version.Get()
		fmt.Printf("treeos version %s\n", versionInfo.Version)
		fmt.Printf("  commit: %s\n", versionInfo.Commit)
		fmt.Printf("  built: %s\n", versionInfo.BuildDate)
		fmt.Printf("  go: %s\n", versionInfo.GoVersion)
		fmt.Printf("  platform: %s\n", versionInfo.Platform)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup-dirs":
			if err := setupDirs(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "migrate-to-compose":
			// Initialize database for migration
			if err := database.Initialize(cfg.DatabasePath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
				os.Exit(1)
			}
			defer func() {
				if err := database.Close(); err != nil {
					log.Printf("Failed to close database: %v", err)
				}
			}()

			// Run migration
			if err := migration.MigrateDeployedAppsToCompose(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Initialize file logging ONLY in development mode
	isDevelopment := os.Getenv("TREEOS_ENV") == "development" || os.Getenv("DEBUG") == "true"

	if isDevelopment {
		logDir := "./logs" // Always use local directory in development
		if err := logging.Initialize(logDir); err != nil {
			log.Printf("Warning: Failed to initialize file logging: %v", err)
			// Continue with standard logging to stdout
		} else {
			defer logging.Close()
			log.Printf("Development logging initialized to %s", logDir)
		}
	} else {
		// In production, just use stdout (captured by systemd/Docker/etc)
		log.Printf("Running in production mode - logging to stdout only")
	}

	// Initialize telemetry
	ctx := context.Background()
	shutdown, err := telemetry.InitializeFromEnv(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize telemetry: %v", err)
		// Continue without telemetry
	} else {
		defer func() {
			if err := shutdown(ctx); err != nil {
				log.Printf("Error shutting down telemetry: %v", err)
			}
		}()
	}

	// Initialize database
	if err := database.Initialize(cfg.DatabasePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	// Get version information
	versionInfo := version.Get()

	// Create and start server
	srv, err := server.New(cfg, versionInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}
	defer srv.Shutdown()

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func setupDirs() error {
	// Determine the apps directory path based on platform
	appsDir := getAppsDir()

	// Platform-specific behavior
	switch runtime.GOOS {
	case "linux":
		return setupLinuxDirs(appsDir)
	case "darwin":
		return setupMacOSDirs(appsDir)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func getAppsDir() string {
	// Load configuration to get the apps directory
	cfg, err := config.Load()
	if err != nil {
		// Fall back to platform defaults if config fails to load
		switch runtime.GOOS {
		case "linux":
			return "/opt/ontree/apps"
		case "darwin":
			return "./apps"
		default:
			return "./apps"
		}
	}
	return cfg.AppsDir
}

func setupLinuxDirs(appsDir string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("This command must be run as root (use sudo)")
	}

	fmt.Printf("Setting up directories for Linux (apps_dir=%s)\n", appsDir)

	// Create parent directory first
	parentDir := filepath.Dir(appsDir)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	// Create apps directory
	if err := os.MkdirAll(appsDir, 0750); err != nil {
		return fmt.Errorf("failed to create apps directory %s: %w", appsDir, err)
	}

	// Try to set ownership to ontreenode:ontreenode
	uid, gid, err := getOntreenodeIDs()
	if err != nil {
		// Fall back to current user
		fmt.Println("Warning: ontreenode user not found, using current user")
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		uid, err = strconv.Atoi(currentUser.Uid)
		if err != nil {
			return fmt.Errorf("failed to parse UID: %w", err)
		}
		gid, err = strconv.Atoi(currentUser.Gid)
		if err != nil {
			return fmt.Errorf("failed to parse GID: %w", err)
		}
	}

	// Set ownership
	if err := os.Chown(appsDir, uid, gid); err != nil {
		return fmt.Errorf("failed to set ownership on %s: %w", appsDir, err)
	}

	// Set permissions to 0775 (group-writable)
	if err := os.Chmod(appsDir, 0750); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", appsDir, err)
	}

	// If running under sudo, add the sudo user to the group
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		groupName := "ontreenode"
		if _, err := user.LookupGroup(groupName); err != nil {
			// If ontreenode group doesn't exist, get the group name of the directory
			if stat, err := os.Stat(appsDir); err == nil {
				if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
					if group, err := user.LookupGroupId(strconv.Itoa(int(sysStat.Gid))); err == nil {
						groupName = group.Name
					}
				}
			}
		}

		// Add user to group
		cmd := exec.Command("usermod", "-a", "-G", groupName, sudoUser)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Could not add %s to %s group: %v\n", sudoUser, groupName, err)
		} else {
			fmt.Printf("Added %s to %s group\n", sudoUser, groupName)
		}
	}

	// Verify write permissions by creating a test file
	testFile := filepath.Join(appsDir, ".test_write")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return fmt.Errorf("failed to verify write permissions in %s: %w", appsDir, err)
	}
	if err := os.Remove(testFile); err != nil {
		// Log but don't fail - the test succeeded
		log.Printf("Warning: failed to remove test file: %v", err)
	}

	fmt.Printf("✓ Successfully created %s with correct permissions\n", appsDir)
	return nil
}

func setupMacOSDirs(appsDir string) error {
	fmt.Printf("Setting up directories for macOS (apps_dir=%s)\n", appsDir)

	// Create apps directory
	if err := os.MkdirAll(appsDir, 0750); err != nil {
		return fmt.Errorf("failed to create apps directory %s: %w", appsDir, err)
	}

	fmt.Printf("✓ Successfully created %s\n", appsDir)
	return nil
}

func getOntreenodeIDs() (int, int, error) {
	// Look up ontreenode user
	u, err := user.Lookup("ontreenode")
	if err != nil {
		return 0, 0, err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, err
	}

	return uid, gid, nil
}
