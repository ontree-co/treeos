package server

import (
	"database/sql"
	"net/http"
	"ontree-node/internal/database"
	"strings"
)

// SetupRequiredMiddleware checks if initial setup is complete
func (s *Server) SetupRequiredMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// URLs that should be accessible without setup
		allowedPaths := []string{
			"/setup",
			"/static/",
			"/patterns/",
		}

		// Check if current path is allowed
		pathAllowed := false
		for _, path := range allowedPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				pathAllowed = true
				break
			}
		}

		if !pathAllowed {
			// Check if any users exist
			db := database.GetDB()
			var userCount int
			err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
			if err != nil {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}

			// Check setup status
			var setupComplete bool
			err = db.QueryRow("SELECT is_setup_complete FROM system_setup WHERE id = 1").Scan(&setupComplete)
			if err != nil && err != sql.ErrNoRows {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}

			// If no users exist or setup is incomplete, redirect to setup
			if userCount == 0 || !setupComplete {
				http.Redirect(w, r, "/setup", http.StatusFound)
				return
			}
		}

		next(w, r)
	}
}

// AuthRequiredMiddleware checks if user is authenticated
func (s *Server) AuthRequiredMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// URLs that should be accessible without authentication
		publicPaths := []string{
			"/login",
			"/logout",
			"/setup",
			"/static/",
			"/patterns/",
		}

		// Check if current path is public
		pathPublic := false
		for _, path := range publicPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				pathPublic = true
				break
			}
		}

		if !pathPublic {
			// Check if user is authenticated
			session, err := s.sessionStore.Get(r, "ontree-session")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			userID, ok := session.Values["user_id"].(int)
			if !ok || userID == 0 {
				// Save the original URL for redirect after login
				session.Values["next"] = r.URL.Path
				session.Save(r, w)
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Load user data and add to context
			user, err := s.getUserByID(userID)
			if err != nil {
				// Invalid session, clear it
				delete(session.Values, "user_id")
				session.Save(r, w)
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Store user in request context
			r = r.WithContext(setUserContext(r.Context(), user))
		}

		next(w, r)
	}
}