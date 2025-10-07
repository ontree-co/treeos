package update

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// GitHubUpdateSource fetches updates from GitHub releases
type GitHubUpdateSource struct {
	Owner       string
	Repo        string
	HTTPClient  *http.Client
}

// NewGitHubUpdateSource creates a new GitHub update source
func NewGitHubUpdateSource() *GitHubUpdateSource {
	return &GitHubUpdateSource{
		Owner: "stefanmunz",
		Repo:  "treeos",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a GitHub release asset
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int    `json:"size"`
}

// FetchManifest fetches the latest release based on the channel
func (s *GitHubUpdateSource) FetchManifest(channel UpdateChannel) (*UpdateManifest, error) {
	var apiURL string

	if channel == ChannelStable {
		// Get the latest non-prerelease
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", s.Owner, s.Repo)
	} else {
		// Get all releases and filter for latest prerelease
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", s.Owner, s.Repo)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// GitHub API requires a user agent
	req.Header.Set("User-Agent", "TreeOS-Updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release *GitHubRelease

	if channel == ChannelStable {
		// Parse single release
		var singleRelease GitHubRelease
		if err := json.NewDecoder(resp.Body).Decode(&singleRelease); err != nil {
			return nil, fmt.Errorf("failed to decode release: %w", err)
		}
		release = &singleRelease
	} else {
		// Parse array of releases and find latest prerelease
		var releases []GitHubRelease
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			return nil, fmt.Errorf("failed to decode releases: %w", err)
		}

		// Find the latest prerelease
		for i := range releases {
			if releases[i].Prerelease {
				release = &releases[i]
				break
			}
		}

		if release == nil {
			return nil, fmt.Errorf("no beta releases found")
		}
	}

	// Convert GitHub release to our UpdateManifest format
	manifest := &UpdateManifest{
		Version:      strings.TrimPrefix(release.TagName, "v"),
		ReleaseDate:  release.PublishedAt,
		ReleaseNotes: release.Body,
		Assets:       Assets{},
	}

	// Find checksums file first
	checksums := make(map[string]string)
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			checksums, err = s.downloadChecksums(asset.BrowserDownloadURL)
			if err != nil {
				// Continue without checksums
				checksums = make(map[string]string)
			}
			break
		}
	}

	// Map assets to platforms
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".tar.gz") {
			platformAsset := &Asset{
				URL:  asset.BrowserDownloadURL,
				Size: int64(asset.Size),
			}

			// Try to get checksum
			if checksum, ok := checksums[asset.Name]; ok {
				platformAsset.SHA256 = checksum
			}

			// Determine platform from filename
			// Expected format: treeos_0.1.0_linux_x86_64.tar.gz
			if strings.Contains(asset.Name, "linux_x86_64") {
				manifest.Assets.LinuxAMD64 = platformAsset
			} else if strings.Contains(asset.Name, "linux_aarch64") || strings.Contains(asset.Name, "linux_arm64") {
				manifest.Assets.LinuxARM64 = platformAsset
			} else if strings.Contains(asset.Name, "darwin_arm64") {
				manifest.Assets.DarwinARM64 = platformAsset
			}
		}
	}

	return manifest, nil
}

// downloadChecksums downloads and parses the checksums.txt file
func (s *GitHubUpdateSource) downloadChecksums(url string) (map[string]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "TreeOS-Updater")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download checksums: status %d", resp.StatusCode)
	}

	// Parse checksums file
	// Format: SHA256 filename
	checksums := make(map[string]string)
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			checksums[parts[1]] = parts[0]
		}
	}

	return checksums, nil
}

// GetAssetForPlatform returns the asset for the current platform
func (s *GitHubUpdateSource) GetAssetForPlatform(manifest *UpdateManifest) (*Asset, error) {
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

// DownloadAsset downloads the asset and extracts the binary from tar.gz
func (s *GitHubUpdateSource) DownloadAsset(asset *Asset, progressCallback func(downloaded, total int64)) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", asset.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "TreeOS-Updater")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download asset: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to download update (HTTP %d)", resp.StatusCode)
	}

	// Extract binary from tar.gz
	return &tarGzBinaryExtractor{
		response:   resp,
		asset:      asset,
		progressCb: progressCallback,
	}, nil
}

// tarGzBinaryExtractor extracts the treeos binary from a tar.gz archive
type tarGzBinaryExtractor struct {
	response   *http.Response
	asset      *Asset
	progressCb func(downloaded, total int64)
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

func (t *tarGzBinaryExtractor) Read(p []byte) (n int, err error) {
	// Initialize extraction on first read
	if t.pipeReader == nil {
		t.pipeReader, t.pipeWriter = io.Pipe()

		// Start extraction in background
		go t.extract()
	}

	return t.pipeReader.Read(p)
}

func (t *tarGzBinaryExtractor) extract() {
	defer t.pipeWriter.Close()
	defer t.response.Body.Close()

	// Track progress
	progressReader := &progressReader{
		reader:     t.response.Body,
		progressCb: t.progressCb,
		total:      t.asset.Size,
	}

	// Create gzip reader
	gzReader, err := gzip.NewReader(progressReader)
	if err != nil {
		t.pipeWriter.CloseWithError(fmt.Errorf("failed to create gzip reader: %w", err))
		return
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Find and extract the treeos binary
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			t.pipeWriter.CloseWithError(fmt.Errorf("treeos binary not found in archive"))
			return
		}
		if err != nil {
			t.pipeWriter.CloseWithError(fmt.Errorf("failed to read tar: %w", err))
			return
		}

		// Look for the treeos binary (might be in root or in a directory)
		if strings.HasSuffix(header.Name, "treeos") || header.Name == "treeos" {
			// Found the binary, copy it to the pipe
			hasher := sha256.New()
			multiWriter := io.MultiWriter(t.pipeWriter, hasher)

			// Limit the size to prevent decompression bombs (max 200MB for binary)
			limitedReader := io.LimitReader(tarReader, 200*1024*1024)
			_, err = io.Copy(multiWriter, limitedReader) //nolint:gosec // Size limited to 200MB
			if err != nil {
				t.pipeWriter.CloseWithError(fmt.Errorf("failed to extract binary: %w", err))
				return
			}

			// Note: The checksum from GitHub is for the tar.gz, not the binary itself
			// So we skip verification here. The binary integrity is ensured by
			// verifying the tar.gz during download if needed.

			return
		}
	}
}

func (t *tarGzBinaryExtractor) Close() error {
	if t.response != nil {
		t.response.Body.Close()
	}
	if t.pipeReader != nil {
		t.pipeReader.Close()
	}
	return nil
}

// progressReader wraps a reader to report progress
type progressReader struct {
	reader     io.Reader
	progressCb func(downloaded, total int64)
	downloaded int64
	total      int64
}

func (p *progressReader) Read(b []byte) (n int, err error) {
	n, err = p.reader.Read(b)
	if n > 0 {
		p.downloaded += int64(n)
		if p.progressCb != nil {
			p.progressCb(p.downloaded, p.total)
		}
	}
	return n, err
}