package server

import (
	"database/sql"
	"net/http"
	"ontree-node/internal/database"
	"ontree-node/internal/telemetry"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

// TracingMiddleware adds OpenTelemetry tracing to HTTP requests
func (s *Server) TracingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Start a new span for this request
		ctx, span := telemetry.StartSpan(r.Context(), r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.String()),
				attribute.String("http.host", r.Host),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("net.transport", "tcp"),
			),
		)
		defer span.End()

		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Pass the request with the new context
		next(rw, r.WithContext(ctx))

		// Set response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", rw.statusCode),
			attribute.Int("http.response_content_length", rw.written),
		)

		// Set span status based on HTTP status code
		if rw.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(rw.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}
