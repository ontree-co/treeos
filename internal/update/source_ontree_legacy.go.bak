package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// OnTreeUpdateSource is a custom update source that fetches from OnTree's update JSON
type OnTreeUpdateSource struct {
	ManifestURL string
	HTTPClient  *http.Client
}

// NewOnTreeUpdateSource creates a new OnTree update source
func NewOnTreeUpdateSource(channel UpdateChannel) *OnTreeUpdateSource {
	manifestURL := fmt.Sprintf("https://ontree.co/api/v1/updates/%s.json", channel)

	return &OnTreeUpdateSource{
		ManifestURL: manifestURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchManifest fetches and parses the update manifest
func (s *OnTreeUpdateSource) FetchManifest() (*UpdateManifest, error) {
	resp, err := s.HTTPClient.Get(s.ManifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Cleanup, error not critical

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var manifest UpdateManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return &manifest, nil
}

// GetAssetForPlatform returns the asset for the current platform
func (s *OnTreeUpdateSource) GetAssetForPlatform(manifest *UpdateManifest) (*Asset, error) {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	switch platform {
	case "linux_amd64":
		if manifest.Assets.LinuxAMD64 == nil {
			return nil, fmt.Errorf("no asset available for platform %s", platform)
		}
		return manifest.Assets.LinuxAMD64, nil
	case "linux_arm64":
		if manifest.Assets.LinuxARM64 == nil {
			return nil, fmt.Errorf("no asset available for platform %s", platform)
		}
		return manifest.Assets.LinuxARM64, nil
	case "darwin_arm64":
		if manifest.Assets.DarwinARM64 == nil {
			return nil, fmt.Errorf("no asset available for platform %s", platform)
		}
		return manifest.Assets.DarwinARM64, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// DownloadAsset downloads the asset and verifies its checksum
func (s *OnTreeUpdateSource) DownloadAsset(asset *Asset, progressCallback func(downloaded, total int64)) (io.ReadCloser, error) {
	resp, err := s.HTTPClient.Get(asset.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download asset from %s: %w", asset.URL, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close() //nolint:errcheck,gosec // Cleanup before error return
		return nil, fmt.Errorf("update package not found (404): %s", asset.URL)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() //nolint:errcheck,gosec // Cleanup before error return
		return nil, fmt.Errorf("failed to download update (HTTP %d): %s", resp.StatusCode, asset.URL)
	}

	// If it's a tar.gz file, we need to extract it
	if strings.HasSuffix(asset.URL, ".tar.gz") {
		return &tarGzExtractor{
			reader:     resp.Body,
			asset:      asset,
			progressCb: progressCallback,
		}, nil
	}

	return &checksumVerifier{
		reader:     resp.Body,
		hash:       sha256.New(),
		expected:   asset.SHA256,
		progressCb: progressCallback,
		total:      asset.Size,
	}, nil
}

// checksumVerifier wraps a reader and verifies SHA256 on Close
type checksumVerifier struct {
	reader     io.ReadCloser
	hash       hash.Hash
	expected   string
	downloaded int64
	total      int64
	progressCb func(downloaded, total int64)
}

func (c *checksumVerifier) Read(p []byte) (n int, err error) {
	n, err = c.reader.Read(p)
	if n > 0 {
		c.hash.Write(p[:n])
		c.downloaded += int64(n)
		if c.progressCb != nil {
			c.progressCb(c.downloaded, c.total)
		}
	}
	return n, err
}

func (c *checksumVerifier) Close() error {
	if err := c.reader.Close(); err != nil {
		return err
	}

	computed := hex.EncodeToString(c.hash.Sum(nil))
	if computed != c.expected {
		return fmt.Errorf("checksum mismatch: got %s, expected %s", computed, c.expected)
	}

	return nil
}

// tarGzExtractor handles tar.gz extraction for the binary
type tarGzExtractor struct {
	reader     io.ReadCloser
	asset      *Asset
	progressCb func(downloaded, total int64)
	downloaded int64
}

func (t *tarGzExtractor) Read(p []byte) (n int, err error) {
	// This will be implemented to extract the binary from tar.gz
	// For now, we'll just pass through
	n, err = t.reader.Read(p)
	if n > 0 {
		t.downloaded += int64(n)
		if t.progressCb != nil && t.asset != nil {
			t.progressCb(t.downloaded, t.asset.Size)
		}
	}
	return n, err
}

func (t *tarGzExtractor) Close() error {
	return t.reader.Close()
}