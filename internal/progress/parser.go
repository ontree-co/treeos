// Package progress provides progress parsing and tracking for Docker operations.
package progress

import (
	"regexp"
	"strconv"
	"strings"
)

// DockerProgressParser parses progress information from Docker compose output
type DockerProgressParser struct {
	tracker *Tracker
}

// NewDockerProgressParser creates a new parser
func NewDockerProgressParser(tracker *Tracker) *DockerProgressParser {
	return &DockerProgressParser{
		tracker: tracker,
	}
}

// PodmanProgressParser is a compatibility alias for DockerProgressParser.
type PodmanProgressParser = DockerProgressParser

// NewPodmanProgressParser creates a new Docker progress parser (compatibility alias).
func NewPodmanProgressParser(tracker *Tracker) *DockerProgressParser {
	return NewDockerProgressParser(tracker)
}

// Progress patterns that we need to match in Docker output:
var (
	// Downloading patterns - enhanced to capture image names
	downloadProgressRegex = regexp.MustCompile(`(?i)(.*?)\s*downloading.*?(\d+(?:\.\d+)?)\s*([KMGT]?B)\s*/\s*(\d+(?:\.\d+)?)\s*([KMGT]?B)`)
	downloadPercentRegex  = regexp.MustCompile(`(?i)(.*?)\s*downloading.*?(\d+(?:\.\d+)?)%`)

	// Image pulling patterns - more comprehensive
	pullingImageRegex = regexp.MustCompile(`(?i)pulling.*?(?:image\s+)?([a-zA-Z0-9._/-]+(?::[a-zA-Z0-9._-]+)?)`)
	extractingRegex   = regexp.MustCompile(`(?i)extracting.*?([a-zA-Z0-9._/-]+(?::[a-zA-Z0-9._-]+)?)`)

	// General progress bar patterns - with image name capture
	progressBarRegex = regexp.MustCompile(`(.*?)\s*\[([=>\s]*)\]\s*(\d+(?:\.\d+)?)\s*([KMGT]?B)\s*/\s*(\d+(?:\.\d+)?)\s*([KMGT]?B)`)

	// Docker specific patterns
	dockerPullingRegex = regexp.MustCompile(`(?i)(?:pulling\s+from\s+|pull\s+complete\s+for\s+)?([a-zA-Z0-9._/-]+(?::[a-zA-Z0-Z._-]+)?)`)
	dockerLayerRegex   = regexp.MustCompile(`(?i)([a-f0-9]{12,}):\s*(downloading|extracting|pull complete).*?(\d+(?:\.\d+)?)\s*([KMGT]?B)\s*/\s*(\d+(?:\.\d+)?)\s*([KMGT]?B)`)

	// Updated patterns to match actual Docker output format
	dockerProgressBarRegex = regexp.MustCompile(`([a-f0-9]{12,})\s+(Downloading|Extracting)\s+\[[=>\s]*\]\s+(\d+(?:\.\d+)?)\s*([KMGT]?B)\s*/\s*(\d+(?:\.\d+)?)\s*([KMGT]?B)`)
	dockerCompleteRegex    = regexp.MustCompile(`([a-f0-9]{12,})\s+(Download complete|Extracting complete|Pull complete)`)

	// Container status patterns
	containerStartingRegex = regexp.MustCompile(`(?i)starting.*?container.*?([^\s]+)`)
	containerStartedRegex  = regexp.MustCompile(`(?i)started.*?([^\s]+)`)

	// Error patterns
	errorPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)error.*?:`),
		regexp.MustCompile(`(?i)failed.*?:`),
		regexp.MustCompile(`(?i)cannot.*?:`),
	}
)

// ParseLine processes a single line of Docker output
func (p *DockerProgressParser) ParseLine(appName, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Check for errors first
	for _, errorRegex := range errorPatterns {
		if errorRegex.MatchString(line) {
			p.tracker.SetError(appName, line)
			return
		}
	}

	// Try to extract image name from the line
	imageName := p.extractImageName(line)

	// Check for different progress patterns
	if p.parseDownloadProgress(appName, imageName, line) {
		return
	}

	if p.parseProgressBar(appName, imageName, line) {
		return
	}

	if p.parseDockerLayerProgress(appName, line) {
		return
	}

	if p.parseDockerProgressBar(appName, line) {
		return
	}

	if p.parseDockerComplete(appName, line) {
		return
	}

	if p.parseImageStatus(appName, line) {
		return
	}

	if p.parseContainerStatus(appName, line) {
		return
	}

	// If no specific pattern matched, update with generic status
	p.updateGenericStatus(appName, line)
}

// parseDownloadProgress extracts download progress from lines like:
// "image:tag Downloading  123.4MB/456.7MB"
// "image:tag Downloading 45.6%"
func (p *DockerProgressParser) parseDownloadProgress(appName, imageName, line string) bool {
	// Try percentage format first
	if matches := downloadPercentRegex.FindStringSubmatch(line); len(matches) > 2 {
		imageFromLine := strings.TrimSpace(matches[1])
		if imageFromLine != "" && imageName == "" {
			imageName = imageFromLine
		}
		if percent, err := strconv.ParseFloat(matches[2], 64); err == nil {
			if imageName != "" {
				p.tracker.UpdateImageProgress(appName, imageName, 0, 0, "downloading")
			}
			message := "Downloading"
			if imageName != "" {
				message = "Downloading " + imageName
			}
			p.tracker.UpdateOperation(appName, OperationDownloading, percent, message, line)
			return true
		}
	}

	// Try size format
	if matches := downloadProgressRegex.FindStringSubmatch(line); len(matches) > 5 {
		imageFromLine := strings.TrimSpace(matches[1])
		if imageFromLine != "" && imageName == "" {
			imageName = imageFromLine
		}
		downloaded, err1 := parseSize(matches[2], matches[3])
		total, err2 := parseSize(matches[4], matches[5])

		if err1 == nil && err2 == nil && total > 0 {
			percent := float64(downloaded) / float64(total) * 100

			if imageName != "" {
				p.tracker.UpdateImageProgress(appName, imageName, downloaded, total, "downloading")
			}

			message := "Downloading"
			if imageName != "" {
				message = "Downloading " + imageName
			}

			details := formatBytes(downloaded) + " / " + formatBytes(total)
			p.tracker.UpdateOperation(appName, OperationDownloading, percent, message, details)
			return true
		}
	}

	return false
}

// parseProgressBar extracts progress from progress bar format:
// "image:tag [====>    ] 123.4MB/456.7MB"
func (p *DockerProgressParser) parseProgressBar(appName, imageName, line string) bool {
	if matches := progressBarRegex.FindStringSubmatch(line); len(matches) > 6 {
		imageFromLine := strings.TrimSpace(matches[1])
		if imageFromLine != "" && imageName == "" {
			imageName = imageFromLine
		}
		downloaded, err1 := parseSize(matches[3], matches[4])
		total, err2 := parseSize(matches[5], matches[6])

		if err1 == nil && err2 == nil && total > 0 {
			percent := float64(downloaded) / float64(total) * 100

			if imageName != "" {
				p.tracker.UpdateImageProgress(appName, imageName, downloaded, total, "downloading")
			}

			message := "Downloading"
			if imageName != "" {
				message = "Downloading " + imageName
			}

			details := formatBytes(downloaded) + " / " + formatBytes(total)
			p.tracker.UpdateOperation(appName, OperationDownloading, percent, message, details)
			return true
		}
	}

	return false
}

// parseDockerLayerProgress handles Docker layer-specific progress
// like "abc123456789: downloading  12.3MB/45.6MB" or "abc123456789: extracting"
func (p *DockerProgressParser) parseDockerLayerProgress(appName, line string) bool {
	if matches := dockerLayerRegex.FindStringSubmatch(line); len(matches) > 6 {
		layerID := matches[1]
		operation := strings.ToLower(matches[2])
		downloaded, err1 := parseSize(matches[3], matches[4])
		total, err2 := parseSize(matches[5], matches[6])

		if err1 == nil && err2 == nil && total > 0 {
			percent := float64(downloaded) / float64(total) * 100

			// Try to get the current image context
			imageName := p.extractImageName(line)
			if imageName == "" {
				// Use layer ID as fallback image identifier
				imageName = "layer:" + layerID
			}

			status := "downloading"
			switch operation {
			case "extracting":
				status = "extracting"
			case "pull complete":
				status = "complete"
				percent = 100
			}

			p.tracker.UpdateImageProgress(appName, imageName, downloaded, total, status)

			message := "Processing layers"
			if imageName != "" && !strings.HasPrefix(imageName, "layer:") {
				message = "Processing " + imageName
			}

			details := formatBytes(downloaded) + " / " + formatBytes(total)
			p.tracker.UpdateOperation(appName, OperationDownloading, percent, message, details)
			return true
		}
	}

	return false
}

// parseDockerProgressBar handles Docker progress bars in the format:
// "a7629e50b7f5 Downloading [>    ] 7.142MB/2.691GB"
func (p *DockerProgressParser) parseDockerProgressBar(appName, line string) bool {
	if matches := dockerProgressBarRegex.FindStringSubmatch(line); len(matches) >= 7 {
		layerID := matches[1]
		operation := strings.ToLower(matches[2])
		downloaded, err1 := parseSize(matches[3], matches[4])
		total, err2 := parseSize(matches[5], matches[6])

		if err1 == nil && err2 == nil && total > 0 {
			percent := float64(downloaded) / float64(total) * 100

			// Use full layer ID as image name for this type of progress
			imageName := layerID

			status := "downloading"
			if operation == "extracting" {
				status = "extracting"
			}

			p.tracker.UpdateImageProgress(appName, imageName, downloaded, total, status)

			message := "Processing layers"
			details := formatBytes(downloaded) + " / " + formatBytes(total)

			operationType := OperationDownloading
			if operation == "extracting" {
				operationType = OperationExtracting
			}

			p.tracker.UpdateOperation(appName, operationType, percent, message, details)
			return true
		}
	}

	return false
}

// parseDockerComplete handles Docker completion messages like:
// "a7629e50b7f5 Download complete"
func (p *DockerProgressParser) parseDockerComplete(appName, line string) bool {
	if matches := dockerCompleteRegex.FindStringSubmatch(line); len(matches) >= 3 {
		layerID := matches[1]
		status := strings.ToLower(matches[2])

		// Use full layer ID as image name to match the format from parseDockerProgressBar
		imageName := layerID

		// Mark this image as complete
		if strings.Contains(status, "download") {
			p.tracker.UpdateImageProgress(appName, imageName, 1, 1, "complete") // 100% progress
		} else if strings.Contains(status, "extract") {
			p.tracker.UpdateImageProgress(appName, imageName, 1, 1, "complete")
		}

		// Don't update overall operation progress here - let it be calculated from image progress
		return true
	}

	return false
}

// parseImageStatus checks for image-related status updates
func (p *DockerProgressParser) parseImageStatus(appName, line string) bool {
	// Check for pulling
	if matches := pullingImageRegex.FindStringSubmatch(line); len(matches) > 1 {
		imageName := matches[1]
		p.tracker.UpdateImageProgress(appName, imageName, 0, 0, "pulling")
		p.tracker.UpdateOperation(appName, OperationDownloading, 0, "Pulling "+imageName, line)
		return true
	}

	// Check for extracting
	if matches := extractingRegex.FindStringSubmatch(line); len(matches) > 1 {
		imageName := matches[1]
		p.tracker.UpdateImageProgress(appName, imageName, 0, 0, "extracting")
		p.tracker.UpdateOperation(appName, OperationExtracting, 50, "Extracting "+imageName, line)
		return true
	}

	return false
}

// parseContainerStatus checks for container-related status updates
func (p *DockerProgressParser) parseContainerStatus(appName, line string) bool {
	// Check for container starting
	if matches := containerStartingRegex.FindStringSubmatch(line); len(matches) > 1 {
		containerName := matches[1]
		p.tracker.UpdateOperation(appName, OperationStarting, 90, "Starting container "+containerName, line)
		return true
	}

	// Check for container started
	if matches := containerStartedRegex.FindStringSubmatch(line); len(matches) > 1 {
		containerName := matches[1]
		p.tracker.UpdateOperation(appName, OperationStarting, 95, "Started "+containerName, line)
		return true
	}

	return false
}

// updateGenericStatus provides fallback status updates for common patterns
func (p *DockerProgressParser) updateGenericStatus(appName, line string) {
	lower := strings.ToLower(line)

	if strings.Contains(lower, "pulling") {
		p.tracker.UpdateOperation(appName, OperationDownloading, 0, "Pulling images", line)
	} else if strings.Contains(lower, "download") {
		p.tracker.UpdateOperation(appName, OperationDownloading, 0, "Downloading", line)
	} else if strings.Contains(lower, "extract") {
		p.tracker.UpdateOperation(appName, OperationExtracting, 50, "Extracting", line)
	} else if strings.Contains(lower, "starting") {
		p.tracker.UpdateOperation(appName, OperationStarting, 80, "Starting containers", line)
	} else if strings.Contains(lower, "creating") {
		p.tracker.UpdateOperation(appName, OperationStarting, 70, "Creating containers", line)
	}
}

// extractImageName attempts to extract an image name from the log line
func (p *DockerProgressParser) extractImageName(line string) string {
	// Try Docker pulling patterns first
	if matches := dockerPullingRegex.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}

	// Try pulling/extracting patterns
	if matches := pullingImageRegex.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	if matches := extractingRegex.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}

	// Try various other patterns to extract image names
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`([a-zA-Z0-9._/-]+/[a-zA-Z0-9._/-]+:[a-zA-Z0-9._-]+)`), // registry/namespace/image:tag
		regexp.MustCompile(`([a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+:[a-zA-Z0-9._-]+)`),   // registry/image:tag
		regexp.MustCompile(`([a-zA-Z0-9._-]+:[a-zA-Z0-9._-]+)`),                   // image:tag
		regexp.MustCompile(`([a-zA-Z0-9._/-]+)`),                                  // simple image name
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
			candidate := matches[1]
			// Filter out common false positives
			if len(candidate) > 3 && !isLikelyFalsePositive(candidate) {
				return candidate
			}
		}
	}

	return ""
}

// isLikelyFalsePositive filters out strings that are unlikely to be image names
func isLikelyFalsePositive(candidate string) bool {
	falsePositives := []string{
		"http", "https", "www", "com", "org", "net", "io",
		"downloading", "extracting", "pulling", "complete",
		"error", "failed", "cannot", "unable",
	}

	lower := strings.ToLower(candidate)
	for _, fp := range falsePositives {
		if strings.Contains(lower, fp) {
			return true
		}
	}

	// Check if it's just a hex string (likely a layer ID)
	if len(candidate) >= 12 && isHexString(candidate) {
		return true
	}

	return false
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return len(s) > 0
}

// parseSize converts size strings like "123.4MB" to bytes
func parseSize(sizeStr, unit string) (int64, error) {
	size, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, err
	}

	multiplier := int64(1)
	switch strings.ToUpper(unit) {
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "B", "":
		multiplier = 1
	}

	return int64(size * float64(multiplier)), nil
}

// formatBytes formats bytes for display
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	if bytes >= GB {
		return formatFloat(float64(bytes)/GB) + "GB"
	} else if bytes >= MB {
		return formatFloat(float64(bytes)/MB) + "MB"
	} else if bytes >= KB {
		return formatFloat(float64(bytes)/KB) + "KB"
	}
	return formatInt64(bytes) + "B"
}

// formatFloat formats a float64 with appropriate precision
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return formatInt64(int64(f))
	}
	// Simple formatting for one decimal place
	intPart := int64(f)
	fracPart := int((f - float64(intPart)) * 10)
	return formatInt64(intPart) + "." + string(rune('0'+fracPart))
}

// formatInt64 formats an int64 to string
func formatInt64(i int64) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + formatInt64(-i)
	}

	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}
