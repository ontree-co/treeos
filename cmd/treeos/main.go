// Package main is the entry point for the OnTree server application
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
			logging.Errorf("No .env file found or error loading it: %v", err)
		}
	}
	logging.ConfigureLevelFromEnv()

	// Parse CLI flags before handling subcommands
	demoMode := false
	showHelp := false
	var portOverride string

	filteredArgs := []string{os.Args[0]}
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-p=") {
			portOverride = strings.TrimPrefix(arg, "-p=")
			continue
		}
		if strings.HasPrefix(arg, "--port=") {
			portOverride = strings.TrimPrefix(arg, "--port=")
			continue
		}
		switch arg {
		case "--demo":
			demoMode = true
		case "-p", "--port":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: -p/--port requires a port value")
				os.Exit(1)
			}
			i++
			portOverride = args[i]
		case "--help", "-h":
			showHelp = true
		case "--version", "-version", "version":
			filteredArgs = append(filteredArgs, arg)
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}
	os.Args = filteredArgs

	// Set run mode environment variable based on flag
	if demoMode {
		os.Setenv("TREEOS_RUN_MODE", "demo") //nolint:errcheck,gosec // Test setup
	}

	if portOverride != "" {
		addr, err := normalizeListenAddr(portOverride)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid port value '%s': %v\n", portOverride, err)
			os.Exit(1)
		}
		os.Setenv("LISTEN_ADDR", addr) //nolint:errcheck,gosec // Config override
	}

	if showHelp {
		printHelp()
		return
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
					logging.Errorf("Failed to close database: %v", err)
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

	// Initialize file logging ONLY in debug mode or demo mode
	isDebug := os.Getenv("DEBUG") == "true"
	isDemo := os.Getenv("TREEOS_RUN_MODE") == "demo"

	if isDebug || isDemo {
		logDir := "./logs" // Always use local directory in debug/demo
		if err := logging.Initialize(logDir); err != nil {
			logging.Warnf("Warning: Failed to initialize file logging: %v", err)
			// Continue with standard logging to stdout
		} else {
			defer logging.Close() //nolint:errcheck // Cleanup, error not critical
			logging.Infof("Debug/demo logging initialized to %s", logDir)
		}
	} else {
		// In production, just use stdout (captured by systemd/launchd/etc)
		logging.Infof("Running in production mode - logging to stdout only")
	}

	// Initialize telemetry only when not in errors-only mode
	ctx := context.Background()
	var shutdown func(context.Context) error
	if logging.GetLevel() < logging.LevelError {
		shutdown, err = telemetry.InitializeFromEnv(ctx)
		if err != nil {
			logging.Warnf("Warning: Failed to initialize telemetry: %v", err)
			// Continue without telemetry
		} else {
			defer func() {
				if err := shutdown(ctx); err != nil {
					logging.Errorf("Error shutting down telemetry: %v", err)
				}
			}()
		}
	} else {
		logging.Infof("Telemetry disabled (LOG_LEVEL=error)")
	}

	// Database initialization is now handled in server.New()
	// to avoid double initialization and ensure proper migration

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
	// Determine the apps directory path based on configuration
	appsDir := getAppsDir()

	// Check if we're in demo mode
	isDemo := os.Getenv("TREEOS_RUN_MODE") == "demo"

	if isDemo {
		// Demo mode: create directories locally without special permissions
		return setupDemoDirs(appsDir)
	}

	// Production mode: platform-specific behavior
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
		// Fall back to centralized path function if config fails to load
		return config.GetAppsPath()
	}
	return cfg.AppsDir
}

func setupDemoDirs(appsDir string) error {
	fmt.Printf("Setting up directories for demo mode (apps_dir=%s)\n", appsDir)

	// Create apps directory
	if err := os.MkdirAll(appsDir, 0750); err != nil {
		return fmt.Errorf("failed to create apps directory %s: %w", appsDir, err)
	}

	// Create shared directory for Ollama models
	sharedDir := "./shared/ollama"
	if err := os.MkdirAll(sharedDir, 0750); err != nil {
		return fmt.Errorf("failed to create shared directory %s: %w", sharedDir, err)
	}

	fmt.Printf("✓ Successfully created directories for demo mode\n")
	return nil
}

func printHelp() {
	fmt.Println("Usage: treeos [options] [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup-dirs            Prepare required directories on the host")
	fmt.Println("  migrate-to-compose    Convert existing deployments to Docker Compose")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help, -h            Show this help message")
	fmt.Println("  --version             Show version information")
	fmt.Println("  --demo                Run using local demo directories")
	fmt.Println("  -p, --port <port>     Override the HTTP listen port (e.g., 4001 or :4001)")
}

func normalizeListenAddr(value string) (string, error) {
	// Allow complete addresses like 127.0.0.1:4000 or [::1]:4000
	if strings.Contains(value, ":") {
		host, port, err := net.SplitHostPort(value)
		if err != nil {
			return "", fmt.Errorf("expected host:port, :port, or [ipv6]:port: %w", err)
		}
		if err := validatePort(port); err != nil {
			return "", err
		}
		if host == "" {
			return ":" + port, nil
		}
		return net.JoinHostPort(host, port), nil
	}

	// Otherwise treat as bare port number
	if err := validatePort(value); err != nil {
		return "", err
	}
	return fmt.Sprintf(":%s", value), nil
}

func validatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port is required")
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func setupLinuxDirs(appsDir string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root (use sudo)")
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
	if err := os.Chmod(appsDir, 0750); err != nil { //nolint:gosec // Directory permissions appropriate for multi-user access
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
		cmd := exec.Command("usermod", "-a", "-G", groupName, sudoUser) //nolint:gosec // Admin command with validated inputs
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Could not add %s to %s group: %v\n", sudoUser, groupName, err)
		} else {
			fmt.Printf("Added %s to %s group\n", sudoUser, groupName)
		}
	}

	// Verify write permissions by creating a test file
	testFile := filepath.Join(appsDir, ".test_write")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil { //nolint:gosec // Test file with secure permissions
		return fmt.Errorf("failed to verify write permissions in %s: %w", appsDir, err)
	}
	if err := os.Remove(testFile); err != nil {
		// Log but don't fail - the test succeeded
		logging.Warnf("Warning: failed to remove test file: %v", err)
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
