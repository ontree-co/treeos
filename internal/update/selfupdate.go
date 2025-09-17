package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/minio/selfupdate"
	"treeos/internal/version"
)

// Service handles self-update operations
type Service struct {
	currentVersion string
	updateChannel  UpdateChannel
	source         *OnTreeUpdateSource
}

// NewService creates a new update service
func NewService(channel UpdateChannel) *Service {
	versionInfo := version.Get()
	return &Service{
		currentVersion: versionInfo.Version,
		updateChannel:  channel,
		source:         NewOnTreeUpdateSource(channel),
	}
}

// SetChannel changes the update channel
func (s *Service) SetChannel(channel UpdateChannel) {
	s.updateChannel = channel
	s.source = NewOnTreeUpdateSource(channel)
	log.Printf("Update channel changed to: %s", channel)
}

// GetChannel returns the current update channel
func (s *Service) GetChannel() UpdateChannel {
	return s.updateChannel
}

// CheckForUpdate checks if an update is available
func (s *Service) CheckForUpdate() (*UpdateInfo, error) {
	log.Printf("Checking for updates on channel: %s", s.updateChannel)

	manifest, err := s.source.FetchManifest()
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
	manifest, err := s.source.FetchManifest()
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
	binaryData, err := s.downloadAndExtractBinary(asset, func(downloaded, total int64) {
		if progressCallback != nil && total > 0 {
			percentage := float64(downloaded) / float64(total) * 100
			progressCallback("downloading", percentage,
				fmt.Sprintf("Downloading... %d/%d bytes", downloaded, total))
		}
	})
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	if progressCallback != nil {
		progressCallback("verifying", 90, "Verifying checksum...")
	}

	// Verify checksum
	if err := s.verifyChecksum(binaryData, asset.SHA256); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

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
func (s *Service) downloadAndExtractBinary(asset *Asset, progressCallback func(downloaded, total int64)) ([]byte, error) {
	resp, err := s.source.HTTPClient.Get(asset.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download from %s: %w", asset.URL, err)
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("update package not found (404): %s", asset.URL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download update (HTTP %d): %s", resp.StatusCode, asset.URL)
	}

	// Read with progress tracking
	var buf bytes.Buffer
	downloaded := int64(0)
	reader := io.TeeReader(resp.Body, &buf)

	// Track progress while downloading
	progressBuf := make([]byte, 32*1024) // 32KB chunks
	for {
		n, err := reader.Read(progressBuf)
		if n > 0 {
			downloaded += int64(n)
			if progressCallback != nil {
				progressCallback(downloaded, asset.Size)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("download error: %w", err)
		}
	}

	// Extract binary from tar.gz if needed
	if strings.HasSuffix(asset.URL, ".tar.gz") {
		return s.extractBinaryFromTarGz(&buf)
	}

	return buf.Bytes(), nil
}

// extractBinaryFromTarGz extracts the treeos binary from a tar.gz archive
func (s *Service) extractBinaryFromTarGz(data io.Reader) ([]byte, error) {
	gzReader, err := gzip.NewReader(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for the treeos binary (could be just "treeos" or in a subdirectory)
		if header.Typeflag == tar.TypeReg {
			name := filepath.Base(header.Name)
			if name == "treeos" || name == "treeos.exe" {
				binary, err := io.ReadAll(tarReader)
				if err != nil {
					return nil, fmt.Errorf("failed to read binary from tar: %w", err)
				}
				return binary, nil
			}
		}
	}

	return nil, fmt.Errorf("treeos binary not found in archive")
}

// verifyChecksum verifies the SHA256 checksum of the downloaded binary
func (s *Service) verifyChecksum(data []byte, expectedSum string) error {
	hash := sha256.Sum256(data)
	actualSum := hex.EncodeToString(hash[:])

	if actualSum != expectedSum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSum, actualSum)
	}

	return nil
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

	// Simple string comparison for now
	// TODO: Implement semantic version comparison
	return latest > current
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
	data, err := os.ReadFile(execPath)
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
	data, err := os.ReadFile(backupPath)
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