package server

import (
	"net/http"
	"strings"
)

// routePatterns routes all /patterns/* requests to the appropriate handler
func (s *Server) routePatterns(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	
	// Route based on the path pattern
	switch {
	case path == "/patterns" || path == "/patterns/":
		s.handlePatternsIndex(w, r)
	case strings.HasSuffix(path, "/components"):
		s.handlePatternsComponents(w, r)
	case strings.HasSuffix(path, "/forms"):
		s.handlePatternsForms(w, r)
	case strings.HasSuffix(path, "/typography"):
		s.handlePatternsTypography(w, r)
	case strings.HasSuffix(path, "/partials"):
		s.handlePatternsPartials(w, r)
	case strings.HasSuffix(path, "/layouts"):
		s.handlePatternsLayouts(w, r)
	case strings.HasSuffix(path, "/style-guide"):
		s.handlePatternsStyleGuide(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handlePatternsIndex handles the pattern library index page
func (s *Server) handlePatternsIndex(w http.ResponseWriter, r *http.Request) {
	// Get user from context if authenticated
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	// Prepare template data
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Pattern Library",
		CSRFToken:   "", // No CSRF yet
		Messages:    nil,
	}
	
	// Render template
	tmpl, ok := s.templates["patterns_index"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

// handlePatternsComponents handles the components pattern page
func (s *Server) handlePatternsComponents(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	// Sample data for components demonstration
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		// Component examples
		Alerts []struct {
			Type    string
			Message string
		}
		Buttons []struct {
			Type string
			Text string
		}
		Cards []struct {
			Title   string
			Content string
		}
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Components - Pattern Library",
		CSRFToken:   "",
		Messages:    nil,
		Alerts: []struct {
			Type    string
			Message string
		}{
			{Type: "success", Message: "Success! This is a success alert."},
			{Type: "danger", Message: "Error! This is a danger alert."},
			{Type: "warning", Message: "Warning! This is a warning alert."},
			{Type: "info", Message: "Info! This is an info alert."},
		},
		Buttons: []struct {
			Type string
			Text string
		}{
			{Type: "primary", Text: "Primary"},
			{Type: "secondary", Text: "Secondary"},
			{Type: "success", Text: "Success"},
			{Type: "danger", Text: "Danger"},
			{Type: "warning", Text: "Warning"},
			{Type: "info", Text: "Info"},
			{Type: "light", Text: "Light"},
			{Type: "dark", Text: "Dark"},
		},
		Cards: []struct {
			Title   string
			Content string
		}{
			{Title: "Basic Card", Content: "This is a basic card with some example content."},
			{Title: "Another Card", Content: "Cards are flexible content containers."},
		},
	}
	
	tmpl, ok := s.templates["patterns_components"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

// handlePatternsForms handles the forms pattern page
func (s *Server) handlePatternsForms(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Forms - Pattern Library",
		CSRFToken:   "",
	}
	
	tmpl, ok := s.templates["patterns_forms"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

// handlePatternsTypography handles the typography pattern page
func (s *Server) handlePatternsTypography(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Typography - Pattern Library",
		CSRFToken:   "",
	}
	
	tmpl, ok := s.templates["patterns_typography"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

// handlePatternsPartials handles the partials pattern page
func (s *Server) handlePatternsPartials(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		// Sample data for partials
		Breadcrumbs []struct {
			Name string
			URL  string
		}
		StatusBadges []struct {
			Status string
			Text   string
		}
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Partials - Pattern Library",
		CSRFToken:   "",
		Messages:    nil,
		Breadcrumbs: []struct {
			Name string
			URL  string
		}{
			{Name: "Dashboard", URL: "/"},
			{Name: "Apps", URL: "/apps"},
			{Name: "My App", URL: "#"},
		},
		StatusBadges: []struct {
			Status string
			Text   string
		}{
			{Status: "running", Text: "Running"},
			{Status: "stopped", Text: "Stopped"},
			{Status: "error", Text: "Error"},
		},
	}
	
	tmpl, ok := s.templates["patterns_partials"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

// handlePatternsLayouts handles the layouts pattern page
func (s *Server) handlePatternsLayouts(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Layouts - Pattern Library",
		CSRFToken:   "",
	}
	
	tmpl, ok := s.templates["patterns_layouts"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

// handlePatternsStyleGuide handles the style guide pattern page
func (s *Server) handlePatternsStyleGuide(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Handle when user is not authenticated
	userInitial := "?"
	if user != nil && user.Username != "" {
		userInitial = getUserInitial(user.Username)
	}
	
	data := struct {
		User        interface{}
		UserInitial string
		Title       string
		CSRFToken   string
		// Style guide data
		Colors []struct {
			Name  string
			Class string
			Hex   string
		}
		Spacing []struct {
			Name  string
			Class string
			Size  string
		}
		Icons []struct {
			Emoji string
			Name  string
		}
		Messages    []interface{}
	}{
		User:        user,
		UserInitial: userInitial,
		Title:       "Style Guide - Pattern Library",
		CSRFToken:   "",
		Messages:    nil,
		Colors: []struct {
			Name  string
			Class string
			Hex   string
		}{
			{Name: "Primary", Class: "primary", Hex: "#007bff"},
			{Name: "Secondary", Class: "secondary", Hex: "#6c757d"},
			{Name: "Success", Class: "success", Hex: "#28a745"},
			{Name: "Danger", Class: "danger", Hex: "#dc3545"},
			{Name: "Warning", Class: "warning", Hex: "#ffc107"},
			{Name: "Info", Class: "info", Hex: "#17a2b8"},
		},
		Spacing: []struct {
			Name  string
			Class string
			Size  string
		}{
			{Name: "Extra Small", Class: "p-1", Size: "0.25rem"},
			{Name: "Small", Class: "p-2", Size: "0.5rem"},
			{Name: "Medium", Class: "p-3", Size: "1rem"},
			{Name: "Large", Class: "p-4", Size: "1.5rem"},
			{Name: "Extra Large", Class: "p-5", Size: "3rem"},
		},
		Icons: []struct {
			Emoji string
			Name  string
		}{
			{Emoji: "🏠", Name: "Home"},
			{Emoji: "📦", Name: "Apps"},
			{Emoji: "⚙️", Name: "Settings"},
			{Emoji: "📊", Name: "Dashboard"},
			{Emoji: "🎨", Name: "Templates"},
			{Emoji: "🔒", Name: "Security"},
		},
	}
	
	tmpl, ok := s.templates["patterns_style_guide"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Error rendering template: " + err.Error(), http.StatusInternalServerError)
		return
	}
}