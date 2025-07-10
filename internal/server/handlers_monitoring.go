package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	
	"ontree-node/internal/charts"
	"ontree-node/internal/database"
	"ontree-node/internal/system"
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
	// Get current CPU usage
	vitals, err := system.GetVitals()
	if err != nil {
		log.Printf("Failed to get system vitals: %v", err)
		http.Error(w, "Failed to get system vitals", http.StatusInternalServerError)
		return
	}
	
	// Get historical CPU data for the last 24 hours
	historicalData, err := database.GetMetricsLast24Hours("cpu")
	if err != nil {
		log.Printf("Failed to get historical CPU data: %v", err)
		// Continue with empty historical data
		historicalData = []database.SystemVitalLog{}
	}
	
	// Extract CPU percentages for sparkline
	var cpuData []float64
	for _, metric := range historicalData {
		cpuData = append(cpuData, metric.CPUPercent)
	}
	
	// Generate sparkline SVG
	var sparklineSVG template.HTML
	if len(cpuData) >= 2 {
		sparklineSVG = charts.GeneratePercentageSparkline(cpuData, 150, 40)
	} else {
		// Not enough data, show a flat line at current value
		sparklineSVG = template.HTML(fmt.Sprintf(`<svg width="150" height="40" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><line x1="0" y1="%d" x2="150" y2="%d" stroke="#007bff" stroke-width="2" /></svg>`, 
			int(40-(vitals.CPUPercent*0.4)), int(40-(vitals.CPUPercent*0.4))))
	}
	
	// Prepare data for the template
	data := struct {
		CurrentLoad  string
		SparklineSVG template.HTML
	}{
		CurrentLoad:  fmt.Sprintf("%.1f", vitals.CPUPercent),
		SparklineSVG: sparklineSVG,
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
	// Get current memory usage
	vitals, err := system.GetVitals()
	if err != nil {
		log.Printf("Failed to get system vitals: %v", err)
		http.Error(w, "Failed to get system vitals", http.StatusInternalServerError)
		return
	}
	
	// Get historical memory data for the last 24 hours
	historicalData, err := database.GetMetricsLast24Hours("memory")
	if err != nil {
		log.Printf("Failed to get historical memory data: %v", err)
		// Continue with empty historical data
		historicalData = []database.SystemVitalLog{}
	}
	
	// Extract memory percentages for sparkline
	var memData []float64
	for _, metric := range historicalData {
		memData = append(memData, metric.MemoryPercent)
	}
	
	// Generate sparkline SVG
	var sparklineSVG template.HTML
	if len(memData) >= 2 {
		sparklineSVG = charts.GeneratePercentageSparkline(memData, 150, 40)
	} else {
		// Not enough data, show a flat line at current value
		sparklineSVG = template.HTML(fmt.Sprintf(`<svg width="150" height="40" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><line x1="0" y1="%d" x2="150" y2="%d" stroke="#007bff" stroke-width="2" /></svg>`, 
			int(40-(vitals.MemPercent*0.4)), int(40-(vitals.MemPercent*0.4))))
	}
	
	// Prepare data for the template
	data := struct {
		CurrentUsage string
		SparklineSVG template.HTML
	}{
		CurrentUsage: fmt.Sprintf("%.1f", vitals.MemPercent),
		SparklineSVG: sparklineSVG,
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
	// Get current disk usage
	vitals, err := system.GetVitals()
	if err != nil {
		log.Printf("Failed to get system vitals: %v", err)
		http.Error(w, "Failed to get system vitals", http.StatusInternalServerError)
		return
	}
	
	// Get historical disk data for the last 24 hours
	historicalData, err := database.GetMetricsLast24Hours("disk")
	if err != nil {
		log.Printf("Failed to get historical disk data: %v", err)
		// Continue with empty historical data
		historicalData = []database.SystemVitalLog{}
	}
	
	// Extract disk percentages for sparkline
	var diskData []float64
	for _, metric := range historicalData {
		diskData = append(diskData, metric.DiskUsagePercent)
	}
	
	// Generate sparkline SVG
	var sparklineSVG template.HTML
	if len(diskData) >= 2 {
		sparklineSVG = charts.GeneratePercentageSparkline(diskData, 150, 40)
	} else {
		// Not enough data, show a flat line at current value
		sparklineSVG = template.HTML(fmt.Sprintf(`<svg width="150" height="40" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><line x1="0" y1="%d" x2="150" y2="%d" stroke="#007bff" stroke-width="2" /></svg>`, 
			int(40-(vitals.DiskPercent*0.4)), int(40-(vitals.DiskPercent*0.4))))
	}
	
	// Prepare data for the template
	data := struct {
		Path         string
		CurrentUsage string
		SparklineSVG template.HTML
	}{
		Path:         "/",
		CurrentUsage: fmt.Sprintf("%.1f", vitals.DiskPercent),
		SparklineSVG: sparklineSVG,
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
	// For network data, we'll use placeholder data for now since the database doesn't store network metrics
	// In a future ticket, we could add network bytes to the database and calculate rates
	
	// Generate a simple placeholder sparkline
	sparklineSVG := template.HTML(`<svg width="150" height="40" viewBox="0 0 150 40" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><polyline fill="none" stroke="#007bff" stroke-width="2" points="0,35 30,30 60,25 90,28 120,32 150,30" /></svg>`)
	
	// Prepare data for the template
	// In a real implementation, we would:
	// 1. Store network bytes in/out in the database
	// 2. Calculate rate by comparing current and previous values
	// 3. Format the rates appropriately
	data := struct {
		DownloadRate string
		UploadRate   string
		SparklineSVG template.HTML
	}{
		DownloadRate: "0 KB/s",  // Placeholder for now
		UploadRate:   "0 KB/s",  // Placeholder for now
		SparklineSVG: sparklineSVG,
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