package server

import (
	"html/template"
	"log"
	"net/http"
	"strings"
)

// handleMonitoring handles the main monitoring dashboard page
func (s *Server) handleMonitoring(w http.ResponseWriter, r *http.Request) {
	// Only handle exact path match
	if r.URL.Path != "/monitoring" {
		http.NotFound(w, r)
		return
	}

	// Get user from context
	user := getUserFromContext(r.Context())

	// Prepare template data
	data := s.baseTemplateData(user)
	data["Title"] = "System Monitoring"

	// Get the monitoring template
	tmpl, ok := s.templates["monitoring"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Error rendering monitoring template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// routeMonitoring routes all /monitoring/* requests to the appropriate handler
func (s *Server) routeMonitoring(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	switch {
	case path == "/monitoring/partials/cpu":
		s.handleMonitoringCPUPartial(w, r)
	case path == "/monitoring/partials/memory":
		s.handleMonitoringMemoryPartial(w, r)
	case path == "/monitoring/partials/disk":
		s.handleMonitoringDiskPartial(w, r)
	case path == "/monitoring/partials/network":
		s.handleMonitoringNetworkPartial(w, r)
	case strings.HasPrefix(path, "/monitoring/charts/"):
		s.handleMonitoringCharts(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleMonitoringCPUPartial returns the CPU monitoring card partial
func (s *Server) handleMonitoringCPUPartial(w http.ResponseWriter, r *http.Request) {
	// Prepare data for the template
	data := struct {
		CurrentLoad  string
		SparklineSVG template.HTML
	}{
		CurrentLoad:  "15.2",
		SparklineSVG: template.HTML(`<svg width="100%" height="100%" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><polyline fill="none" stroke="#007bff" stroke-width="2" points="0,30 30,25 60,20 90,22 120,18 150,15" /></svg>`),
	}

	// Get the CPU card template
	tmpl, ok := s.templates["_cpu_card"]
	if !ok {
		log.Printf("CPU card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "cpu-card-partial", data); err != nil {
		log.Printf("Error rendering CPU card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleMonitoringMemoryPartial returns the memory monitoring card partial
func (s *Server) handleMonitoringMemoryPartial(w http.ResponseWriter, r *http.Request) {
	// Prepare data for the template
	data := struct {
		CurrentUsage string
		SparklineSVG template.HTML
	}{
		CurrentUsage: "45.8",
		SparklineSVG: template.HTML(`<svg width="100%" height="100%" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><polyline fill="none" stroke="#007bff" stroke-width="2" points="0,20 30,22 60,25 90,23 120,20 150,18" /></svg>`),
	}

	// Get the memory card template
	tmpl, ok := s.templates["_memory_card"]
	if !ok {
		log.Printf("Memory card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "memory-card-partial", data); err != nil {
		log.Printf("Error rendering memory card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleMonitoringDiskPartial returns the disk monitoring card partial
func (s *Server) handleMonitoringDiskPartial(w http.ResponseWriter, r *http.Request) {
	// Prepare data for the template
	data := struct {
		Path         string
		CurrentUsage string
		SparklineSVG template.HTML
	}{
		Path:         "/",
		CurrentUsage: "78.1",
		SparklineSVG: template.HTML(`<svg width="100%" height="100%" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><polyline fill="none" stroke="#007bff" stroke-width="2" points="0,10 30,10 60,11 90,11 120,12 150,12" /></svg>`),
	}

	// Get the disk card template
	tmpl, ok := s.templates["_disk_card"]
	if !ok {
		log.Printf("Disk card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "disk-card-partial", data); err != nil {
		log.Printf("Error rendering disk card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleMonitoringNetworkPartial returns the network monitoring card partial
func (s *Server) handleMonitoringNetworkPartial(w http.ResponseWriter, r *http.Request) {
	// Prepare data for the template
	data := struct {
		DownloadRate string
		UploadRate   string
		SparklineSVG template.HTML
	}{
		DownloadRate: "1.2 MB/s",
		UploadRate:   "85 KB/s",
		SparklineSVG: template.HTML(`<svg width="100%" height="100%" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><polyline fill="none" stroke="#007bff" stroke-width="2" points="0,35 30,30 60,25 90,28 120,32 150,30" /></svg>`),
	}

	// Get the network card template
	tmpl, ok := s.templates["_network_card"]
	if !ok {
		log.Printf("Network card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "network-card-partial", data); err != nil {
		log.Printf("Error rendering network card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleMonitoringCharts returns detailed charts for specific metrics
func (s *Server) handleMonitoringCharts(w http.ResponseWriter, r *http.Request) {
	// Extract metric type from path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	
	metricType := parts[3]
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `
<div class="modal-chart">
    <h2>` + strings.Title(metricType) + ` Details</h2>
    <p>Detailed chart for ` + metricType + ` coming soon...</p>
    <svg width="600" height="300" viewBox="0 0 600 300" xmlns="http://www.w3.org/2000/svg">
        <!-- Detailed chart will go here -->
        <rect x="0" y="0" width="600" height="300" fill="#f8f9fa" stroke="#dee2e6" />
        <text x="300" y="150" text-anchor="middle" fill="#6c757d">Detailed ` + strings.Title(metricType) + ` Chart</text>
    </svg>
</div>
	`
	w.Write([]byte(html))
}