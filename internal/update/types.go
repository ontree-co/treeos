package update

import (
	"time"
)

// UpdateChannel represents the update channel (stable or beta)
type UpdateChannel string

const (
	ChannelStable UpdateChannel = "stable"
	ChannelBeta   UpdateChannel = "beta"
)

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	UpdateAvailable bool     `json:"update_available"`
	ReleaseNotes   string    `json:"release_notes,omitempty"`
	ReleaseDate    time.Time `json:"release_date,omitempty"`
	DownloadURL    string    `json:"download_url,omitempty"`
	DownloadSize   int64     `json:"download_size,omitempty"`
	SHA256         string    `json:"sha256,omitempty"`
}

// UpdateManifest represents the JSON structure from the update server
type UpdateManifest struct {
	Version      string    `json:"version"`
	ReleaseDate  time.Time `json:"release_date"`
	ReleaseNotes string    `json:"release_notes"`
	Assets       Assets    `json:"assets"`
}

// Assets contains platform-specific download information
type Assets struct {
	LinuxAMD64  *Asset `json:"linux_amd64,omitempty"`
	LinuxARM64  *Asset `json:"linux_arm64,omitempty"`
	DarwinARM64 *Asset `json:"darwin_arm64,omitempty"`
}

// Asset represents a downloadable binary asset
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// UpdateProgress represents the progress of an update operation
type UpdateProgress struct {
	Stage      string  `json:"stage"` // "checking", "downloading", "verifying", "applying"
	Percentage float64 `json:"percentage"`
	Message    string  `json:"message"`
}