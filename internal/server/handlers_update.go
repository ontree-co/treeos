package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// ImageUpdateInfo contains information about available image updates
type ImageUpdateInfo struct {
	CurrentImage    string `json:"current_image"`
	CurrentDigest   string `json:"current_digest"`
	LatestDigest    string `json:"latest_digest"`
	UpdateAvailable bool   `json:"update_available"`
	Error           string `json:"error,omitempty"`
}

// PullStatus represents the status of a docker pull operation
type PullStatus struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

// handleAppCheckUpdate checks for available updates for a specific service's Docker image
func (s *Server) handleAppCheckUpdate(w http.ResponseWriter, r *http.Request) {
	// Extract app name from path
	appName := strings.TrimPrefix(r.URL.Path, "/apps/")
	appName = strings.TrimSuffix(appName, "/check-update")

	// Get service name from query parameter
	serviceName := r.URL.Query().Get("service")

	log.Printf("Checking for updates for app: %s, service: %s", appName, serviceName)

	// Get app details
	app, err := s.dockerSvc.GetAppDetails(appName)
	if err != nil {
		log.Printf("Failed to get app details: %v", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-danger">Failed to get app details</span>`)
		return
	}

	if app == nil {
		log.Printf("App %s not found", appName)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-danger">App not found</span>`)
		return
	}

	// If no service specified, return error
	if serviceName == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-danger">Service name required</span>`)
		return
	}

	// Find the service
	service, found := app.Services[serviceName]
	if !found {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-danger">Service %s not found</span>`, serviceName)
		return
	}

	imageName := service.Image
	if imageName == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-muted">No image configured</span>`)
		return
	}

	// Check for updates
	updateInfo, err := s.checkImageUpdate(imageName)
	if err != nil {
		log.Printf("Failed to check updates for image %s: %v", imageName, err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-danger">Failed to check: %s</span>`, err.Error())
		return
	}

	// Extract current version/tag
	currentTag := "latest"
	if strings.Contains(imageName, ":") {
		parts := strings.Split(imageName, ":")
		currentTag = parts[1]
	}

	// Prepare response based on update availability
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if updateInfo.UpdateAvailable {
		// Show update available with version info
		fmt.Fprintf(w, `
			<div class="d-flex align-items-center gap-2">
				<span class="text-warning">
					<i>⬆️</i> Update available
				</span>
				<span class="text-muted small">%s → newer version</span>
				<button class="btn btn-sm btn-warning" 
					onclick="recreateService('%s', '%s')">
					Recreate
				</button>
			</div>`, currentTag, appName, serviceName)
	} else {
		// Show current version and up-to-date status
		fmt.Fprintf(w, `
			<div class="d-flex align-items-center gap-2">
				<span class="text-success">
					<i>✓</i> Up to date
				</span>
				<span class="text-muted small">(%s)</span>
			</div>`, currentTag)
	}
}

// checkImageUpdate checks if there's an update available for the given image
func (s *Server) checkImageUpdate(imageName string) (*ImageUpdateInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	updateInfo := &ImageUpdateInfo{
		CurrentImage: imageName,
	}

	// Get current image digest
	imageList, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	// Find the current image
	for _, img := range imageList {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				if len(img.RepoDigests) > 0 {
					updateInfo.CurrentDigest = img.RepoDigests[0]
				}
				break
			}
		}
	}

	// Pull the latest version to check for updates
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull latest image: %w", err)
	}
	defer reader.Close()

	// Read the pull output to ensure it completes
	pullOutput, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("Error reading pull output: %v", err)
	}

	// Check if the pull output indicates a new image was downloaded
	pullOutputStr := string(pullOutput)
	var updateDetected bool
	if strings.Contains(pullOutputStr, "Downloaded newer image") || strings.Contains(pullOutputStr, "Pull complete") {
		updateDetected = true
	}

	// Get the new image digest
	imageList, err = cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images after pull: %w", err)
	}

	// Find the pulled image
	for _, img := range imageList {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				if len(img.RepoDigests) > 0 {
					updateInfo.LatestDigest = img.RepoDigests[0]
				}
				break
			}
		}
	}

	// Compare digests
	if updateInfo.CurrentDigest != "" && updateInfo.LatestDigest != "" {
		updateInfo.UpdateAvailable = updateInfo.CurrentDigest != updateInfo.LatestDigest
	} else {
		// If we couldn't get digests, use the pull output detection
		updateInfo.UpdateAvailable = updateDetected
	}

	return updateInfo, nil
}

// handleAppUpdate handles the actual update of an app's Docker image
func (s *Server) handleAppUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from path
	appName := strings.TrimPrefix(r.URL.Path, "/apps/")
	appName = strings.TrimSuffix(appName, "/update")

	log.Printf("Update requested for app: %s", appName)

	// For now, just redirect back with a message that the user needs to recreate manually
	session, err := s.sessionStore.Get(r, "ontree-session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
	} else {
		session.AddFlash("Please use the 'Recreate' button to update the container with the latest image", "info")
		if err := session.Save(r, w); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/apps/%s", appName), http.StatusSeeOther)
}
