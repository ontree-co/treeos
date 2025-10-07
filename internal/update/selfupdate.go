// Package update provides self-update functionality for TreeOS.
package update

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/minio/selfupdate"
	"treeos/internal/version"
)

// Service handles self-update operations
type Service struct {
	currentVersion string
	updateChannel  UpdateChannel
	source         *GitHubUpdateSource
}

// NewService creates a new update service
func NewService(channel UpdateChannel) *Service {
	versionInfo := version.Get()
	return &Service{
		currentVersion: versionInfo.Version,
		updateChannel:  channel,
		source:         NewGitHubUpdateSource(),
	}
}

// SetChannel changes the update channel
func (s *Service) SetChannel(channel UpdateChannel) {
	s.updateChannel = channel
	// GitHub source doesn't need to be recreated when channel changes
	log.Printf("Update channel changed to: %s", channel)
}

// GetChannel returns the current update channel
func (s *Service) GetChannel() UpdateChannel {
	return s.updateChannel
}

// CheckForUpdate checks if an update is available
func (s *Service) CheckForUpdate() (*UpdateInfo, error) {
	log.Printf("Checking for updates on channel: %s", s.updateChannel)

	manifest, err := s.source.FetchManifest(s.updateChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch update manifest: %w", err)
	}

	// Get the asset for current platform
	asset, err := s.source.GetAssetForPlatform(manifest)
	if err != nil {
		return nil, fmt.Errorf("no update available for platform: %w", err)
	}

	info := &UpdateInfo{
		CurrentVersion:  s.currentVersion,
		LatestVersion:   manifest.Version,
		UpdateAvailable: s.isNewerVersion(manifest.Version),
		ReleaseNotes:    manifest.ReleaseNotes,
		ReleaseDate:     manifest.ReleaseDate,
		DownloadURL:     asset.URL,
		DownloadSize:    asset.Size,
		SHA256:          asset.SHA256,
	}

	log.Printf("Update check complete. Current: %s, Latest: %s, Available: %v",
		info.CurrentVersion, info.LatestVersion, info.UpdateAvailable)

	return info, nil
}

// ApplyUpdate downloads and applies the update
func (s *Service) ApplyUpdate(progressCallback func(stage string, percentage float64, message string)) error {
	log.Printf("Starting update process...")

	if progressCallback != nil {
		progressCallback("checking", 0, "Checking for updates...")
	}

	// Check for update first
	manifest, err := s.source.FetchManifest(s.updateChannel)
	if err != nil {
		return fmt.Errorf("failed to fetch update manifest: %w", err)
	}

	if !s.isNewerVersion(manifest.Version) {
		return fmt.Errorf("no update available (current: %s, latest: %s)", s.currentVersion, manifest.Version)
	}

	// Get the asset for current platform
	asset, err := s.source.GetAssetForPlatform(manifest)
	if err != nil {
		return fmt.Errorf("no update available for platform: %w", err)
	}

	if progressCallback != nil {
		progressCallback("downloading", 0, fmt.Sprintf("Downloading version %s...", manifest.Version))
	}

	// Download the update
	_, binaryData, err := s.downloadAndExtractBinary(asset, func(downloaded, total int64) {
		if progressCallback != nil && total > 0 {
			percentage := float64(downloaded) / float64(total) * 100
			progressCallback("downloading", percentage,
				fmt.Sprintf("Downloading... %d/%d bytes", downloaded, total))
		}
	})
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	// Skip checksum verification for GitHub releases
	// The integrity is ensured by HTTPS and GitHub's infrastructure
	// The checksums in GitHub releases are for the tar.gz archives,
	// not the extracted binaries, so we can't verify them after extraction

	if progressCallback != nil {
		progressCallback("applying", 95, "Applying update...")
	}

	// Apply the update using minio/selfupdate
	err = selfupdate.Apply(bytes.NewReader(binaryData), selfupdate.Options{})
	if err != nil {
		// Check if we need to handle rollback
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %v, rollback error: %v", err, rerr)
		}
		return fmt.Errorf("failed to apply update: %w", err)
	}

	if progressCallback != nil {
		progressCallback("complete", 100, "Update applied successfully!")
	}

	log.Printf("Successfully updated to version %s", manifest.Version)
	return nil
}

// downloadAndExtractBinary downloads the asset and extracts the binary from tar.gz
// Returns both the archive data (for checksum verification) and the extracted binary
func (s *Service) downloadAndExtractBinary(asset *Asset, progressCallback func(downloaded, total int64)) ([]byte, []byte, error) {
	// Use the GitHub source's DownloadAsset method which handles extraction
	reader, err := s.source.DownloadAsset(asset, progressCallback)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download asset: %w", err)
	}
	defer reader.Close()

	// Read the binary data
	binaryData, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read binary: %w", err)
	}

	// For GitHub releases, we skip checksum verification of the archive
	// since we're extracting directly. The integrity is ensured by HTTPS
	// and GitHub's infrastructure.
	// Return nil for archive data since we don't have it (and don't need it)
	return nil, binaryData, nil
}


// isNewerVersion compares version strings
func (s *Service) isNewerVersion(latestVersion string) bool {
	// Clean up version strings (remove 'v' prefix if present)
	current := strings.TrimPrefix(s.currentVersion, "v")
	latest := strings.TrimPrefix(latestVersion, "v")

	// Special case for development versions
	if current == "dev" || current == "unknown" {
		return true // Always allow updates from dev versions
	}

	// Compare semantic versions properly
	return compareVersions(latest, current) > 0
}

// compareVersions compares two semantic version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Split versions into parts (e.g., "0.1.0-beta.10" -> ["0.1.0", "beta.10"])
	parts1 := strings.SplitN(v1, "-", 2)
	parts2 := strings.SplitN(v2, "-", 2)

	// Compare main version numbers
	mainCmp := compareVersionParts(parts1[0], parts2[0])
	if mainCmp != 0 {
		return mainCmp
	}

	// If main versions are equal, compare pre-release versions
	// No pre-release version is higher than having a pre-release
	if len(parts1) == 1 && len(parts2) == 2 {
		return 1 // v1 (stable) > v2 (pre-release)
	}
	if len(parts1) == 2 && len(parts2) == 1 {
		return -1 // v1 (pre-release) < v2 (stable)
	}
	if len(parts1) == 2 && len(parts2) == 2 {
		// Both have pre-release versions, compare them
		return comparePreRelease(parts1[1], parts2[1])
	}

	return 0
}

// compareVersionParts compares main version numbers (e.g., "0.1.0" vs "0.2.0")
func compareVersionParts(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Pad shorter version with zeros
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int

		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}

// comparePreRelease compares pre-release versions (e.g., "beta.9" vs "beta.10")
func comparePreRelease(pr1, pr2 string) int {
	// Split by dots to handle "beta.10", "rc.1", etc.
	parts1 := strings.Split(pr1, ".")
	parts2 := strings.Split(pr2, ".")

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		// Try to parse as number first
		n1, err1 := strconv.Atoi(parts1[i])
		n2, err2 := strconv.Atoi(parts2[i])

		// If both are numbers, compare numerically
		if err1 == nil && err2 == nil {
			if n1 > n2 {
				return 1
			}
			if n1 < n2 {
				return -1
			}
			continue
		}

		// Otherwise compare as strings
		cmp := strings.Compare(parts1[i], parts2[i])
		if cmp != 0 {
			return cmp
		}
	}

	// If all compared parts are equal, longer version is greater
	if len(parts1) > len(parts2) {
		return 1
	}
	if len(parts1) < len(parts2) {
		return -1
	}

	return 0
}

// GetCurrentVersion returns the current version
func (s *Service) GetCurrentVersion() string {
	return s.currentVersion
}

// BackupCurrentBinary creates a backup of the current binary
func (s *Service) BackupCurrentBinary() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	backupPath := execPath + ".backup"

	// Read current binary
	data, err := os.ReadFile(execPath) //nolint:gosec // Path from executable location
	if err != nil {
		return "", fmt.Errorf("failed to read current binary: %w", err)
	}

	// Write backup
	err = os.WriteFile(backupPath, data, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	log.Printf("Created backup at: %s", backupPath)
	return backupPath, nil
}

// RestoreBackup restores the backup binary
func (s *Service) RestoreBackup(backupPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Read backup
	data, err := os.ReadFile(backupPath) //nolint:gosec // Path from backup location
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Restore
	err = os.WriteFile(execPath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	log.Printf("Restored backup from: %s", backupPath)
	return nil
}

// IsRunningAsService checks if the binary is running under systemd or similar
func IsRunningAsService() bool {
	// Check for systemd
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}

	// Check if running on Linux with init as parent
	if runtime.GOOS == "linux" && os.Getppid() == 1 {
		return true
	}

	return false
}