package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/ontree-co/treeos/internal/logging"

	"github.com/ontree-co/treeos/internal/database"
	"github.com/ontree-co/treeos/internal/update"
)

// handleSystemUpdateCheck checks for available system updates
func (s *Server) handleSystemUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	channel := s.getUpdateChannel()

	// Create update service
	updateSvc := update.NewService(channel)

	// Check for updates
	updateInfo, err := updateSvc.CheckForUpdate()
	if err != nil {
		logging.Errorf("Failed to check for updates: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to check for updates",
			"details": err.Error(),
		}); err != nil {
			logging.Errorf("Failed to encode response: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updateInfo); err != nil {
		logging.Errorf("Failed to encode update info: %v", err)
	}
}

// handleSystemUpdateApply applies a system update
func (s *Server) handleSystemUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication - only admins can update
	user := getUserFromContext(r.Context())
	if user == nil || !user.IsStaff {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	channel := s.getUpdateChannel()

	// Create update service
	updateSvc := update.NewService(channel)

	// Record update attempt in history (if table exists)
	var historyID int64
	result, err := s.db.Exec(`
		INSERT INTO update_history (version, channel, status, started_at)
		VALUES (?, ?, 'in_progress', CURRENT_TIMESTAMP)
	`, updateSvc.GetCurrentVersion(), string(channel))

	if err == nil {
		historyID, err = result.LastInsertId()
		if err != nil {
			logging.Errorf("Failed to get last insert ID: %v", err)
		}
	} else if strings.Contains(err.Error(), "no such table") {
		// Table doesn't exist yet, migrations haven't run
		logging.Infof("update_history table doesn't exist, skipping history recording")
	} else {
		logging.Errorf("Failed to record update attempt: %v", err)
	}

	// Reset update status and mark as in progress
	SetUpdateStatus(UpdateStatus{
		InProgress:     true,
		Message:        "Starting update...",
		Stage:          "initializing",
		StartedAt:      time.Now(),
		CurrentVersion: updateSvc.GetCurrentVersion(),
	})

	// Start update in background
	go func() {
		s.updateMu.Lock()
		defer s.updateMu.Unlock()
		// Apply the update
		err := updateSvc.ApplyUpdate(func(stage string, percentage float64, message string) {
			// Log progress
			logging.Infof("Update progress: [%s] %.0f%% - %s", stage, percentage, message)

			// Update status
			SetUpdateStatus(UpdateStatus{
				InProgress:     true,
				Message:        message,
				Stage:          stage,
				Percentage:     percentage,
				StartedAt:      GetUpdateStatus().StartedAt,
				CurrentVersion: updateSvc.GetCurrentVersion(),
			})

			// Could send SSE events here if we implement real-time updates
			if s.sseManager != nil {
				s.sseManager.SendToAll("update-progress", map[string]interface{}{
					"stage":      stage,
					"percentage": percentage,
					"message":    message,
				})
			}
		})

		// Update history record
		if historyID > 0 {
			status := "success"
			var errorMsg *string
			if err != nil {
				status = "failed"
				errStr := err.Error()
				errorMsg = &errStr
			}

			_, updateErr := s.db.Exec(`
				UPDATE update_history
				SET status = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, status, errorMsg, historyID)

			if updateErr != nil {
				logging.Errorf("Failed to update history record: %v", updateErr)
			}
		}

		if err != nil {
			logging.Errorf("Update failed: %v", err)

			// Prepare user-friendly error message
			userMessage := "The update process failed. Please try again later."
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
				userMessage = "Update package not found. The update may not be available yet. Please try again later."
			} else if strings.Contains(err.Error(), "download") || strings.Contains(err.Error(), "network") {
				userMessage = "Failed to download the update. Please check your internet connection and try again."
			} else if strings.Contains(err.Error(), "checksum") {
				userMessage = "Update verification failed. The downloaded file may be corrupted. Please try again."
			} else if strings.Contains(err.Error(), "permission") {
				userMessage = "Permission denied. Please ensure TreeOS has write access to its installation directory."
			}

			// Set error status
			SetUpdateStatus(UpdateStatus{
				InProgress:     false,
				Failed:         true,
				Error:          userMessage,
				Message:        err.Error(), // Technical details in message
				Stage:          "failed",
				CurrentVersion: updateSvc.GetCurrentVersion(),
			})

			if s.sseManager != nil {
				s.sseManager.SendToAll("update-failed", map[string]interface{}{
					"error":   userMessage,
					"details": err.Error(),
				})
			}
			return
		}

		// Set success status
		SetUpdateStatus(UpdateStatus{
			InProgress:       false,
			Success:          true,
			Message:          "Update applied successfully, restarting...",
			Stage:            "complete",
			CurrentVersion:   updateSvc.GetCurrentVersion(),
			AvailableVersion: updateSvc.GetCurrentVersion(),
		})

		logging.Info("Update applied successfully, system will restart...")

		// Send success notification
		if s.sseManager != nil {
			s.sseManager.SendToAll("update-complete", map[string]interface{}{
				"message": "Update complete, restarting...",
			})
		}

		// Give time for the response to be sent and database to settle
		time.Sleep(3 * time.Second)

		// Force database checkpoint before shutdown to ensure WAL is written
		if s.db != nil {
			logging.Info("Performing database checkpoint before restart...")
			if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
				logging.Warnf("Warning: Failed to checkpoint database before restart: %v", err)
			}
			// Don't close database here - Shutdown() will do it
		}

		// Add a bit more time for filesystem to sync
		time.Sleep(2 * time.Second)

		// The systemd service should restart the application
		// Call Shutdown to cleanly close everything
		s.Shutdown()

		// Now exit the process to trigger systemd restart
		// This will also trigger the defer in main.go
		logging.Info("Exiting process to trigger systemd restart...")
		os.Exit(0)
	}()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "Update started",
		"message": "The system will restart automatically after the update is applied",
	}); err != nil {
		logging.Errorf("Failed to encode response: %v", err)
	}
}

// handleSystemUpdateChannel gets or sets the update channel
func (s *Server) handleSystemUpdateChannel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetUpdateChannel(w, r)
	case http.MethodPut:
		s.handleSetUpdateChannel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetUpdateChannel(w http.ResponseWriter, _ *http.Request) {
	var channel string
	err := s.db.QueryRow(`SELECT update_channel FROM system_setup WHERE id = 1`).Scan(&channel)
	if err != nil {
		// Default to stable if not found or column doesn't exist
		channel = "stable"
		if err != sql.ErrNoRows && !strings.Contains(err.Error(), "no such column") {
			logging.Errorf("Failed to get update channel: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"channel": channel,
	}); err != nil {
		logging.Errorf("Failed to encode channel response: %v", err)
	}
}

func (s *Server) handleSetUpdateChannel(w http.ResponseWriter, r *http.Request) {
	// Check authentication - only admins can change channel
	user := getUserFromContext(r.Context())
	if user == nil || !user.IsStaff {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Channel string `json:"channel"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate channel
	if req.Channel != "stable" && req.Channel != "beta" {
		http.Error(w, "Invalid channel, must be 'stable' or 'beta'", http.StatusBadRequest)
		return
	}

	// Update database
	_, err := s.db.Exec(`
		UPDATE system_setup
		SET update_channel = ?
		WHERE id = 1
	`, req.Channel)

	if err != nil {
		// If column doesn't exist, we can't save it but continue anyway
		if strings.Contains(err.Error(), "no such column") {
			logging.Infof("update_channel column doesn't exist yet, will use default")
			// Still return success since the in-memory value can be used
		} else {
			logging.Errorf("Failed to update channel: %v", err)
			http.Error(w, "Failed to update channel", http.StatusInternalServerError)
			return
		}
	}

	logging.Infof("Update channel changed to: %s", req.Channel)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"channel": req.Channel,
	}); err != nil {
		logging.Errorf("Failed to encode response: %v", err)
	}
}

// handleSystemUpdateStatus returns the current update status
func (s *Server) handleSystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := GetUpdateStatus()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		logging.Errorf("Failed to encode status: %v", err)
	}
}

// handleSystemUpdateHistory returns the update history
func (s *Server) handleSystemUpdateHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := s.db.Query(`
		SELECT id, version, channel, status, error_message, started_at, completed_at, created_at
		FROM update_history
		ORDER BY started_at DESC
		LIMIT 20
	`)
	if err != nil {
		logging.Errorf("Failed to query update history: %v", err)
		http.Error(w, "Failed to retrieve update history", http.StatusInternalServerError)
		return
	}
	defer rows.Close() //nolint:errcheck // Cleanup, error not critical

	var history []database.UpdateHistory
	for rows.Next() {
		var h database.UpdateHistory
		err := rows.Scan(&h.ID, &h.Version, &h.Channel, &h.Status,
			&h.ErrorMessage, &h.StartedAt, &h.CompletedAt, &h.CreatedAt)
		if err != nil {
			logging.Errorf("Failed to scan update history row: %v", err)
			continue
		}
		history = append(history, h)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		logging.Errorf("Failed to encode history: %v", err)
	}
}

// handleSystemUpdateRestart triggers a restart if an update has been applied and requires restart
func (s *Server) handleSystemUpdateRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil || !user.IsStaff {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	status := GetUpdateStatus()
	if !status.RestartRequired {
		http.Error(w, "No pending update restart", http.StatusBadRequest)
		return
	}

	SetUpdateStatus(UpdateStatus{
		InProgress:       true,
		Stage:            "restarting",
		Message:          "Restarting to finish update...",
		CurrentVersion:   status.CurrentVersion,
		AvailableVersion: status.AvailableVersion,
	})

	if s.sseManager != nil {
		s.sseManager.SendToAll("update-restarting", map[string]interface{}{
			"version": status.AvailableVersion,
		})
	}

	go func() {
		time.Sleep(2 * time.Second)
		s.Shutdown()
	}()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "restarting",
		"message": "Restarting to complete update...",
	}); err != nil {
		logging.Errorf("Failed to encode restart response: %v", err)
	}
}
