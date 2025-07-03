package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup-dirs" {
		if err := setupDirs(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("Starting server...")
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
	// Check for environment variable override
	if dir := os.Getenv("ONTREE_APPS_DIR"); dir != "" {
		return dir
	}
	
	// Platform-specific defaults
	switch runtime.GOOS {
	case "linux":
		return "/opt/ontree/apps"
	case "darwin":
		return "./apps"
	default:
		return "./apps"
	}
}

func setupLinuxDirs(appsDir string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("This command must be run as root (use sudo)")
	}
	
	fmt.Printf("Setting up directories for Linux (apps_dir=%s)\n", appsDir)
	
	// Create parent directory first
	parentDir := filepath.Dir(appsDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}
	
	// Create apps directory
	if err := os.MkdirAll(appsDir, 0775); err != nil {
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
		uid, _ = strconv.Atoi(currentUser.Uid)
		gid, _ = strconv.Atoi(currentUser.Gid)
	}
	
	// Set ownership
	if err := os.Chown(appsDir, uid, gid); err != nil {
		return fmt.Errorf("failed to set ownership on %s: %w", appsDir, err)
	}
	
	// Set permissions to 0775 (group-writable)
	if err := os.Chmod(appsDir, 0775); err != nil {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("failed to verify write permissions in %s: %w", appsDir, err)
	}
	os.Remove(testFile)
	
	fmt.Printf("✓ Successfully created %s with correct permissions\n", appsDir)
	return nil
}

func setupMacOSDirs(appsDir string) error {
	fmt.Printf("Setting up directories for macOS (apps_dir=%s)\n", appsDir)
	
	// Create apps directory
	if err := os.MkdirAll(appsDir, 0755); err != nil {
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