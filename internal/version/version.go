// Package version provides version information about the application.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

// These variables are set at build time using -ldflags.
// By default, they are set to "dev" or "unknown", which is fine for local development.
var (
	// Version is the git tag version number.
	Version = "dev"
	// Commit is the git commit hash.
	Commit = "unknown"
	// BuildDate is the date of the build.
	BuildDate = "unknown"
)

// Info holds all the version information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
}

// Get returns the version information.
func Get() Info {
	// Start with the information available from ldflags.
	info := Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}

	// Get additional information from debug.ReadBuildInfo
	if bi, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = bi.GoVersion

		// Extract compiler and platform from settings
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "-compiler":
				info.Compiler = setting.Value
			case "GOOS":
				info.Platform = setting.Value
			case "GOARCH":
				if info.Platform != "" {
					info.Platform += "/" + setting.Value
				}
			case "vcs.revision":
				// If Commit is still "unknown", use the one from build info
				if info.Commit == "unknown" {
					info.Commit = setting.Value
				}
			}
		}
	}

	// If platform is still empty, use runtime info
	if info.Platform == "" {
		info.Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return info
}

// GetVersionAge returns a human-readable age of the build.
func GetVersionAge() string {
	if BuildDate == "unknown" {
		return "unknown"
	}

	// Try to parse the build date
	t, err := time.Parse(time.RFC3339, BuildDate)
	if err != nil {
		// Try alternative format
		t, err = time.Parse("2006-01-02T15:04:05Z", BuildDate)
		if err != nil {
			return "unknown"
		}
	}

	duration := time.Since(t)

	// Format the duration in a human-readable way
	if duration < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	} else if duration < 30*24*time.Hour {
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	} else if duration < 365*24*time.Hour {
		return fmt.Sprintf("%d months ago", int(duration.Hours()/(24*30)))
	}
	return fmt.Sprintf("%d years ago", int(duration.Hours()/(24*365)))
}
