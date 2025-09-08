package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

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

	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	case path == "/monitoring/dashboard/all":
		s.handleDashboardMonitoringUpdate(w, r)
	case path == "/monitoring/partials/cpu":
		s.handleMonitoringCPUPartial(w, r)
	case path == "/monitoring/partials/memory":
		s.handleMonitoringMemoryPartial(w, r)
	case path == "/monitoring/partials/disk":
		s.handleMonitoringDiskPartial(w, r)
	case path == "/monitoring/partials/network":
		s.handleMonitoringNetworkPartial(w, r)
	case path == "/monitoring/partials/gpu":
		s.handleMonitoringGPUPartial(w, r)
	case path == "/monitoring/partials/upload":
		s.handleMonitoringUploadPartial(w, r)
	case path == "/monitoring/partials/download":
		s.handleMonitoringDownloadPartial(w, r)
	case strings.HasPrefix(path, "/monitoring/charts/"):
		s.handleMonitoringCharts(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleDashboardMonitoringUpdate returns all six monitoring cards data for the dashboard
// This is called every second via HTMX to update the monitoring cards
func (s *Server) handleDashboardMonitoringUpdate(w http.ResponseWriter, r *http.Request) {
	// Track last update times for memory and disk (update every 60 seconds)
	var memoryValue, diskValue float64
	var memorySparkline, diskSparkline template.HTML

	// Get current timestamp for cache keys
	now := time.Now()
	// Round to minute for memory/disk caching
	minuteKey := now.Truncate(time.Minute).Unix()

	// Get memory data (cached for 60 seconds)
	memoryCacheKey := fmt.Sprintf("dashboard:memory:%d", minuteKey)
	if cached, found := s.sparklineCache.Get(memoryCacheKey); found {
		if data, ok := cached.(map[string]interface{}); ok {
			if val, ok := data["value"].(float64); ok {
				memoryValue = val
			}
			if spark, ok := data["sparkline"].(template.HTML); ok {
				memorySparkline = spark
			}
		}
	} else {
		// Get fresh memory data
		vitals, err := system.GetVitals()
		if err != nil {
			log.Printf("Failed to get vitals: %v", err)
		}
		if vitals != nil {
			memoryValue = vitals.MemPercent
		}
		// Generate memory sparkline from last 24h data
		if historicalData, err := database.GetMetricsLast24Hours("memory"); err == nil && len(historicalData) > 0 {
			points := make([]float64, len(historicalData))
			for i, m := range historicalData {
				points[i] = m.MemoryPercent
			}
			memorySparkline = template.HTML(charts.GenerateSparklineSVG(points, 150, 40))
		}
		// Cache for 60 seconds
		s.sparklineCache.Set(memoryCacheKey, map[string]interface{}{
			"value":     memoryValue,
			"sparkline": memorySparkline,
		})
	}

	// Get disk data (cached for 60 seconds)
	diskCacheKey := fmt.Sprintf("dashboard:disk:%d", minuteKey)
	if cached, found := s.sparklineCache.Get(diskCacheKey); found {
		if data, ok := cached.(map[string]interface{}); ok {
			if val, ok := data["value"].(float64); ok {
				diskValue = val
			}
			if spark, ok := data["sparkline"].(template.HTML); ok {
				diskSparkline = spark
			}
		}
	} else {
		// Get fresh disk data
		vitals, err := system.GetVitals()
		if err != nil {
			log.Printf("Failed to get vitals: %v", err)
		}
		if vitals != nil {
			diskValue = vitals.DiskPercent
		}
		// Generate disk sparkline from last 24h data
		if historicalData, err := database.GetMetricsLast24Hours("disk"); err == nil && len(historicalData) > 0 {
			points := make([]float64, len(historicalData))
			for i, m := range historicalData {
				points[i] = m.DiskUsagePercent
			}
			diskSparkline = template.HTML(charts.GenerateSparklineSVG(points, 150, 40))
		}
		// Cache for 60 seconds
		s.sparklineCache.Set(diskCacheKey, map[string]interface{}{
			"value":     diskValue,
			"sparkline": diskSparkline,
		})
	}

	// Get real-time data for CPU, GPU, and Network (updated every second)
	vitals, err := system.GetVitals()
	if err != nil {
		log.Printf("Failed to get system vitals: %v", err)
		http.Error(w, "Failed to get system vitals", http.StatusInternalServerError)
		return
	}

	// Generate sparklines for real-time metrics (CPU, GPU, Network)
	// These use recent historical data combined with real-time data
	var cpuSparkline, gpuSparkline, uploadSparkline, downloadSparkline template.HTML

	// CPU sparkline
	if historicalData, err := database.GetMetricsLast24Hours("cpu"); err == nil && len(historicalData) > 0 {
		points := make([]float64, len(historicalData))
		for i, m := range historicalData {
			points[i] = m.CPUPercent
		}
		cpuSparkline = template.HTML(charts.GenerateSparklineSVG(points, 150, 40))
	}

	// GPU sparkline
	if historicalData, err := database.GetMetricsLast24Hours("gpu"); err == nil && len(historicalData) > 0 {
		points := make([]float64, len(historicalData))
		for i, m := range historicalData {
			points[i] = m.GPULoad
		}
		gpuSparkline = template.HTML(charts.GenerateSparklineSVG(points, 150, 40))
	}

	// Network sparklines
	if historicalData, err := database.GetMetricsLast24Hours("network"); err == nil && len(historicalData) > 0 {
		uploadPoints := make([]float64, len(historicalData))
		downloadPoints := make([]float64, len(historicalData))
		for i, m := range historicalData {
			uploadPoints[i] = float64(m.UploadRate)
			downloadPoints[i] = float64(m.DownloadRate)
		}
		// Normalize for display
		uploadPoints = normalizeNetworkRates(uploadPoints)
		downloadPoints = normalizeNetworkRates(downloadPoints)
		uploadSparkline = template.HTML(charts.GenerateSparklineSVG(uploadPoints, 150, 40))
		downloadSparkline = template.HTML(charts.GenerateSparklineSVG(downloadPoints, 150, 40))
	}

	// Prepare the response HTML with all six cards
	html := fmt.Sprintf(`
	<div id="monitoring-cards-container">
		<div class="row g-3">
			<!-- First Row: CPU, GPU, Memory -->
			<!-- CPU Card -->
			<div class="col-12 col-md-6 col-lg-4">
				<div id="cpu-card">
					<div class="card monitoring-card bg-white">
						<div class="card-body">
							<h6 class="metric-title">CPU Usage</h6>
							<div class="metric-value">%.1f%%</div>
							<div class="sparkline-container" data-metric="cpu">
								%s
							</div>
						</div>
					</div>
				</div>
			</div>
			
			<!-- GPU Card -->
			<div class="col-12 col-md-6 col-lg-4">
				<div id="gpu-card">
					<div class="card monitoring-card bg-white">
						<div class="card-body">
							<h6 class="metric-title">GPU Load</h6>
							<div class="metric-value">%.1f%%</div>
							<div class="sparkline-container" data-metric="gpu">
								%s
							</div>
						</div>
					</div>
				</div>
			</div>
			
			<!-- Memory Card -->
			<div class="col-12 col-md-6 col-lg-4">
				<div id="memory-card">
					<div class="card monitoring-card bg-white">
						<div class="card-body">
							<h6 class="metric-title">Memory Usage</h6>
							<div class="metric-value">%.1f%%</div>
							<div class="sparkline-container" data-metric="memory">
								%s
							</div>
						</div>
					</div>
				</div>
			</div>
			
			<!-- Second Row: Disk, Download, Upload -->
			<!-- Disk Card -->
			<div class="col-12 col-md-6 col-lg-4">
				<div id="disk-card">
					<div class="card monitoring-card bg-white">
						<div class="card-body">
							<h6 class="metric-title">Disk Usage (/)</h6>
							<div class="metric-value">%.1f%%</div>
							<div class="sparkline-container" data-metric="disk">
								%s
							</div>
						</div>
					</div>
				</div>
			</div>
			
			<!-- Download Card -->
			<div class="col-12 col-md-6 col-lg-4">
				<div id="download-card">
					<div class="card monitoring-card bg-white">
						<div class="card-body">
							<h6 class="metric-title">Download</h6>
							<div class="metric-value">%s</div>
							<div class="sparkline-container" data-metric="download">
								%s
							</div>
						</div>
					</div>
				</div>
			</div>
			
			<!-- Upload Card -->
			<div class="col-12 col-md-6 col-lg-4">
				<div id="upload-card">
					<div class="card monitoring-card bg-white">
						<div class="card-body">
							<h6 class="metric-title">Upload</h6>
							<div class="metric-value">%s</div>
							<div class="sparkline-container" data-metric="upload">
								%s
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>`,
		vitals.CPUPercent, cpuSparkline,
		vitals.GPULoad, gpuSparkline,
		memoryValue, memorySparkline,
		diskValue, diskSparkline,
		formatNetworkRate(float64(vitals.DownloadRate)), downloadSparkline,
		formatNetworkRate(float64(vitals.UploadRate)), uploadSparkline,
	)

	// Return the HTML response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleMonitoringCPUPartial returns the CPU monitoring card partial
func (s *Server) handleMonitoringCPUPartial(w http.ResponseWriter, r *http.Request) {
	// Get current CPU usage from real-time metrics
	var currentCPU float64
	if latest, ok := s.realtimeMetrics.GetLatestCPU(); ok {
		currentCPU = latest.Value
	} else {
		// Fallback to system vitals if no real-time data
		vitals, err := system.GetVitals()
		if err != nil {
			log.Printf("Failed to get system vitals: %v", err)
			http.Error(w, "Failed to get system vitals", http.StatusInternalServerError)
			return
		}
		currentCPU = vitals.CPUPercent
	}

	// Don't cache sparklines for real-time data
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	// Get real-time data for the last 60 seconds
	realtimeData := s.realtimeMetrics.GetCPU(60 * time.Second)

	// Get historical data from database (older than 60 seconds)
	oneMinuteAgo := now.Add(-60 * time.Second)
	historicalData, err := database.GetMetricsForTimeRange(startTime, oneMinuteAgo)
	if err != nil {
		log.Printf("Failed to get historical CPU data: %v", err)
		historicalData = []database.SystemVitalLog{}
	}

	// Combine data: historical + real-time
	var timeSeriesData []charts.TimeSeriesPoint

	// Add historical data
	for _, metric := range historicalData {
		if metric.Timestamp.Before(oneMinuteAgo) {
			timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
				Time:  metric.Timestamp,
				Value: metric.CPUPercent,
			})
		}
	}

	// Add real-time data
	for _, point := range realtimeData {
		timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
			Time:  point.Time,
			Value: point.Value,
		})
	}

	// Generate sparkline SVG with time awareness
	opts := charts.DefaultPercentageOptions()
	sparklineSVG := charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

	// Prepare data for the template
	data := struct {
		CurrentLoad  string
		SparklineSVG template.HTML
	}{
		CurrentLoad:  fmt.Sprintf("%.1f", currentCPU),
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

	// Check cache for sparkline
	cacheKey := "sparkline:memory:24h"
	var sparklineSVG template.HTML

	if cached, found := s.sparklineCache.Get(cacheKey); found {
		if svg, ok := cached.(template.HTML); ok {
			sparklineSVG = svg
		} else {
			log.Printf("Invalid type in sparkline cache for key %s", cacheKey)
		}
	} else {
		// Get historical memory data for the last 24 hours
		now := time.Now()
		startTime := now.Add(-24 * time.Hour)

		historicalData, err := database.GetMetricsLast24Hours("memory")
		if err != nil {
			log.Printf("Failed to get historical memory data: %v", err)
			historicalData = []database.SystemVitalLog{}
		}

		// Convert to time series points
		var timeSeriesData []charts.TimeSeriesPoint
		for _, metric := range historicalData {
			timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
				Time:  metric.Timestamp,
				Value: metric.MemoryPercent,
			})
		}

		// Generate sparkline SVG with time awareness
		opts := charts.DefaultPercentageOptions()
		sparklineSVG = charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

		// Cache the sparkline
		s.sparklineCache.Set(cacheKey, sparklineSVG)
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

	// Check cache for sparkline
	cacheKey := "sparkline:disk:24h"
	var sparklineSVG template.HTML

	if cached, found := s.sparklineCache.Get(cacheKey); found {
		if svg, ok := cached.(template.HTML); ok {
			sparklineSVG = svg
		} else {
			log.Printf("Invalid type in sparkline cache for key %s", cacheKey)
		}
	} else {
		// Get historical disk data for the last 24 hours
		now := time.Now()
		startTime := now.Add(-24 * time.Hour)

		historicalData, err := database.GetMetricsLast24Hours("disk")
		if err != nil {
			log.Printf("Failed to get historical disk data: %v", err)
			historicalData = []database.SystemVitalLog{}
		}

		// Convert to time series points
		var timeSeriesData []charts.TimeSeriesPoint
		for _, metric := range historicalData {
			timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
				Time:  metric.Timestamp,
				Value: metric.DiskUsagePercent,
			})
		}

		// Generate sparkline SVG with time awareness
		opts := charts.DefaultPercentageOptions()
		sparklineSVG = charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

		// Cache the sparkline
		s.sparklineCache.Set(cacheKey, sparklineSVG)
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
	// Get historical network data for rate calculation
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	// Calculate current network rates from real-time data
	var downloadRate, uploadRate string
	realtimeNetData := s.realtimeMetrics.GetNetwork(2 * time.Second) // Get last 2 seconds of data

	if len(realtimeNetData) >= 2 {
		// Calculate rate between most recent two points
		latest := realtimeNetData[len(realtimeNetData)-1]
		previous := realtimeNetData[len(realtimeNetData)-2]

		timeDiff := latest.Time.Sub(previous.Time).Seconds()
		if timeDiff > 0 && timeDiff < 2 { // Real-time data should be within 2 seconds
			// Calculate bytes per second, handling counter resets
			var rxRate, txRate float64

			if latest.RxBytes >= previous.RxBytes {
				rxRate = float64(latest.RxBytes-previous.RxBytes) / timeDiff
			} else {
				// Counter reset or overflow, show 0
				rxRate = 0
			}

			if latest.TxBytes >= previous.TxBytes {
				txRate = float64(latest.TxBytes-previous.TxBytes) / timeDiff
			} else {
				// Counter reset or overflow, show 0
				txRate = 0
			}

			// Format rates
			downloadRate = formatNetworkRate(rxRate)
			uploadRate = formatNetworkRate(txRate)
		} else {
			downloadRate = "0 KB/s"
			uploadRate = "0 KB/s"
		}
	} else {
		// Fallback to database if no real-time data
		recentMetrics, err := database.GetMetricsForTimeRange(now.Add(-2*time.Minute), now)
		if err != nil {
			log.Printf("Failed to get recent network metrics: %v", err)
			recentMetrics = []database.SystemVitalLog{}
		}

		if len(recentMetrics) >= 2 {
			// Calculate rate between most recent two points
			latest := recentMetrics[len(recentMetrics)-1]
			previous := recentMetrics[len(recentMetrics)-2]

			timeDiff := latest.Timestamp.Sub(previous.Timestamp).Seconds()
			if timeDiff > 0 && timeDiff < 120 { // Ignore gaps larger than 2 minutes
				// Calculate bytes per second, handling counter resets
				var rxRate, txRate float64

				// We now store rates directly in bytes per second
				rxRate = float64(latest.DownloadRate)
				txRate = float64(latest.UploadRate)

				// Format rates
				downloadRate = formatNetworkRate(rxRate)
				uploadRate = formatNetworkRate(txRate)
			} else {
				downloadRate = "0 KB/s"
				uploadRate = "0 KB/s"
			}
		} else {
			downloadRate = "0 KB/s"
			uploadRate = "0 KB/s"
		}
	}

	// Get real-time data for the last 60 seconds
	realtimeData := s.realtimeMetrics.GetNetwork(60 * time.Second)

	// Get historical data from database (older than 60 seconds)
	oneMinuteAgo := now.Add(-60 * time.Second)
	historicalData, err := database.GetMetricsForTimeRange(startTime, oneMinuteAgo)
	if err != nil {
		log.Printf("Failed to get historical network data: %v", err)
		historicalData = []database.SystemVitalLog{}
	}

	// Calculate rates for sparkline (MB/s for better scale)
	var timeSeriesData []charts.TimeSeriesPoint

	// Process historical data
	if len(historicalData) > 0 {
		// Add initial zero point
		timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
			Time:  historicalData[0].Timestamp,
			Value: 0,
		})

		for i := 1; i < len(historicalData); i++ {
			prev := historicalData[i-1]
			curr := historicalData[i]

			// Only include data older than 60 seconds
			if curr.Timestamp.Before(oneMinuteAgo) {
				timeDiff := curr.Timestamp.Sub(prev.Timestamp).Seconds()
				if timeDiff > 0 && timeDiff < 120 { // Only use points within 2 minutes of each other
					// Calculate combined rate in MB/s, handling counter resets
					var rxRate, txRate float64

					// We now store rates directly in bytes per second
					rxRate = float64(curr.DownloadRate) / 1024 / 1024 // Convert to MB/s
					txRate = float64(curr.UploadRate) / 1024 / 1024   // Convert to MB/s

					totalRate := rxRate + txRate

					timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
						Time:  curr.Timestamp,
						Value: totalRate,
					})
				}
			}
		}
	}

	// Process real-time data
	if len(realtimeData) > 0 {
		// Add zero point if no historical data
		if len(timeSeriesData) == 0 {
			timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
				Time:  realtimeData[0].Time,
				Value: 0,
			})
		}

		for i := 1; i < len(realtimeData); i++ {
			prev := realtimeData[i-1]
			curr := realtimeData[i]

			timeDiff := curr.Time.Sub(prev.Time).Seconds()
			if timeDiff > 0 && timeDiff < 2 { // Real-time data should be within 2 seconds
				// Calculate combined rate in MB/s, handling counter resets
				var rxRate, txRate float64

				if curr.RxBytes >= prev.RxBytes {
					rxRate = float64(curr.RxBytes-prev.RxBytes) / timeDiff / 1024 / 1024
				}
				if curr.TxBytes >= prev.TxBytes {
					txRate = float64(curr.TxBytes-prev.TxBytes) / timeDiff / 1024 / 1024
				}

				totalRate := rxRate + txRate

				timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
					Time:  curr.Time,
					Value: totalRate,
				})
			}
		}
	}

	// Generate sparkline
	opts := charts.DefaultSparklineOptions()
	opts.ShowNoData = true
	sparklineSVG := charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

	// Prepare data for the template
	data := struct {
		DownloadRate string
		UploadRate   string
		SparklineSVG template.HTML
	}{
		DownloadRate: downloadRate,
		UploadRate:   uploadRate,
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
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract metric type from path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}

	metricType := parts[3]

	// Default time range is 24 hours
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}

	var duration time.Duration

	switch timeRange {
	case "1h":
		duration = time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = 24 * time.Hour
	}

	// Get all metrics in a single batch query
	startTime := time.Now().Add(-duration)
	endTime := time.Now()

	// Check cache for the specific metric and time range
	cacheKey := fmt.Sprintf("chart:%s:%s", metricType, timeRange)
	var chartSVG template.HTML

	if cached, found := s.sparklineCache.Get(cacheKey); found {
		if svg, ok := cached.(template.HTML); ok {
			chartSVG = svg
		} else {
			log.Printf("Invalid type in sparkline cache for key %s", cacheKey)
		}
	} else {
		// Batch query for all metrics
		batch, err := database.GetMetricsBatch(startTime, endTime)
		if err != nil {
			log.Printf("Failed to get metrics batch: %v", err)
			batch = &database.MetricsBatch{Metrics: []database.SystemVitalLog{}}
		}

		// Prepare chart data based on metric type
		var chartData charts.DetailedChartData
		chartData.StartTime = startTime
		chartData.EndTime = endTime

		switch metricType {
		case "cpu":
			chartData.Title = "CPU Usage"
			chartData.YAxisUnit = "%"
			chartData.MinValue = 0
			chartData.MaxValue = 100

			// Convert to DataPoints
			for _, metric := range batch.Metrics {
				chartData.Points = append(chartData.Points, charts.DataPoint{
					Time:  metric.Timestamp,
					Value: metric.CPUPercent,
				})
			}

		case "memory":
			chartData.Title = "Memory Usage"
			chartData.YAxisUnit = "%"
			chartData.MinValue = 0
			chartData.MaxValue = 100

			// Convert to DataPoints
			for _, metric := range batch.Metrics {
				chartData.Points = append(chartData.Points, charts.DataPoint{
					Time:  metric.Timestamp,
					Value: metric.MemoryPercent,
				})
			}

		case "disk":
			chartData.Title = "Disk Usage (/)"
			chartData.YAxisUnit = "%"
			chartData.MinValue = 0
			chartData.MaxValue = 100

			// Convert to DataPoints
			for _, metric := range batch.Metrics {
				chartData.Points = append(chartData.Points, charts.DataPoint{
					Time:  metric.Timestamp,
					Value: metric.DiskUsagePercent,
				})
			}

		case "gpu":
			chartData.Title = "GPU Load"
			chartData.YAxisUnit = "%"
			chartData.MinValue = 0
			chartData.MaxValue = 100

			// Convert to DataPoints
			for _, metric := range batch.Metrics {
				chartData.Points = append(chartData.Points, charts.DataPoint{
					Time:  metric.Timestamp,
					Value: metric.GPULoad,
				})
			}

		case "network":
			chartData.Title = "Network Usage"
			chartData.YAxisUnit = "MB/s"

			// Calculate network rates from consecutive data points
			if len(batch.Metrics) > 1 {
				for i := 1; i < len(batch.Metrics); i++ {
					prev := batch.Metrics[i-1]
					curr := batch.Metrics[i]

					timeDiff := curr.Timestamp.Sub(prev.Timestamp).Seconds()
					if timeDiff > 0 && timeDiff < 120 { // Only use points within 2 minutes
						// We now store rates directly in bytes per second
						rxRate := float64(curr.DownloadRate) / 1024 / 1024 // Convert to MB/s
						txRate := float64(curr.UploadRate) / 1024 / 1024   // Convert to MB/s
						totalRate := rxRate + txRate

						chartData.Points = append(chartData.Points, charts.DataPoint{
							Time:  curr.Timestamp,
							Value: totalRate,
						})
					}
				}
			}

		case "download":
			chartData.Title = "Download Rate"
			chartData.YAxisUnit = "MB/s"

			// Convert to DataPoints
			for _, metric := range batch.Metrics {
				chartData.Points = append(chartData.Points, charts.DataPoint{
					Time:  metric.Timestamp,
					Value: float64(metric.DownloadRate) / 1024 / 1024, // Convert to MB/s
				})
			}

		case "upload":
			chartData.Title = "Upload Rate"
			chartData.YAxisUnit = "MB/s"

			// Convert to DataPoints
			for _, metric := range batch.Metrics {
				chartData.Points = append(chartData.Points, charts.DataPoint{
					Time:  metric.Timestamp,
					Value: float64(metric.UploadRate) / 1024 / 1024, // Convert to MB/s
				})
			}

		default:
			http.NotFound(w, r)
			return
		}

		// Generate the detailed chart
		chartSVG = charts.GenerateDetailedChart(chartData, 700, 400)

		// Cache the generated chart
		s.sparklineCache.Set(cacheKey, chartSVG)
	}

	// Time range selector buttons
	timeRangeButtons := fmt.Sprintf(`
	<div class="btn-group mb-3" role="group" aria-label="Time range selector">
		<button type="button" class="btn btn-sm %s" onclick="loadChart('%s', '1h')">1 Hour</button>
		<button type="button" class="btn btn-sm %s" onclick="loadChart('%s', '6h')">6 Hours</button>
		<button type="button" class="btn btn-sm %s" onclick="loadChart('%s', '24h')">24 Hours</button>
		<button type="button" class="btn btn-sm %s" onclick="loadChart('%s', '7d')">7 Days</button>
	</div>`,
		ifElse(timeRange == "1h", "btn-primary", "btn-outline-primary"), metricType,
		ifElse(timeRange == "6h", "btn-primary", "btn-outline-primary"), metricType,
		ifElse(timeRange == "24h", "btn-primary", "btn-outline-primary"), metricType,
		ifElse(timeRange == "7d", "btn-primary", "btn-outline-primary"), metricType,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`
<div class="modal-chart">
    %s
    <div class="chart-container">
        %s
    </div>
</div>
<script>
function loadChart(metric, range) {
    htmx.ajax('GET', '/monitoring/charts/' + metric + '?range=' + range, {
        target: '#modal-content',
        swap: 'innerHTML'
    });
}
</script>
	`, timeRangeButtons, chartSVG)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// Helper function for conditional strings
func ifElse(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

// handleMonitoringGPUPartial returns the GPU monitoring card partial
func (s *Server) handleMonitoringGPUPartial(w http.ResponseWriter, r *http.Request) {
	// Get latest metric from database
	latest, err := database.GetLatestMetric("")
	if err != nil {
		log.Printf("Failed to get latest GPU metric: %v", err)
		latest = &database.SystemVitalLog{GPULoad: 0}
	}

	// Get historical GPU data for the last 24 hours
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	historicalData, err := database.GetMetricsLast24Hours("gpu")
	if err != nil {
		log.Printf("Failed to get historical GPU data: %v", err)
		historicalData = []database.SystemVitalLog{}
	}

	// Convert to time series points
	var timeSeriesData []charts.TimeSeriesPoint
	for _, metric := range historicalData {
		timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
			Time:  metric.Timestamp,
			Value: metric.GPULoad,
		})
	}

	// Generate sparkline SVG with time awareness
	opts := charts.DefaultPercentageOptions()
	sparklineSVG := charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

	// Prepare data for the template
	data := struct {
		CurrentLoad  string
		SparklineSVG template.HTML
	}{
		CurrentLoad:  fmt.Sprintf("%.1f", latest.GPULoad),
		SparklineSVG: sparklineSVG,
	}

	// Get the GPU card template
	tmpl, ok := s.templates["_gpu_card"]
	if !ok {
		log.Printf("GPU card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "gpu-card-partial", data); err != nil {
		log.Printf("Error rendering GPU card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleMonitoringDownloadPartial returns the download monitoring card partial
func (s *Server) handleMonitoringDownloadPartial(w http.ResponseWriter, r *http.Request) {
	// Get latest metric from database
	latest, err := database.GetLatestMetric("")
	if err != nil {
		log.Printf("Failed to get latest download metric: %v", err)
		latest = &database.SystemVitalLog{DownloadRate: 0}
	}

	// Get historical data for the last 24 hours
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	historicalData, err := database.GetMetricsForTimeRange(startTime, now)
	if err != nil {
		log.Printf("Failed to get historical download data: %v", err)
		historicalData = []database.SystemVitalLog{}
	}

	// Convert to time series points (in MB/s for sparkline)
	var timeSeriesData []charts.TimeSeriesPoint
	for _, metric := range historicalData {
		timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
			Time:  metric.Timestamp,
			Value: float64(metric.DownloadRate) / 1024 / 1024, // Convert to MB/s
		})
	}

	// Generate sparkline SVG
	opts := charts.DefaultSparklineOptions()
	opts.ShowNoData = true
	sparklineSVG := charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

	// Prepare data for the template
	data := struct {
		CurrentRate  string
		SparklineSVG template.HTML
	}{
		CurrentRate:  formatNetworkRate(float64(latest.DownloadRate)),
		SparklineSVG: sparklineSVG,
	}

	// Get the download card template
	tmpl, ok := s.templates["_download_card"]
	if !ok {
		log.Printf("Download card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "download-card-partial", data); err != nil {
		log.Printf("Error rendering download card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// handleMonitoringUploadPartial returns the upload monitoring card partial
func (s *Server) handleMonitoringUploadPartial(w http.ResponseWriter, r *http.Request) {
	// Get latest metric from database
	latest, err := database.GetLatestMetric("")
	if err != nil {
		log.Printf("Failed to get latest upload metric: %v", err)
		latest = &database.SystemVitalLog{UploadRate: 0}
	}

	// Get historical data for the last 24 hours
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	historicalData, err := database.GetMetricsForTimeRange(startTime, now)
	if err != nil {
		log.Printf("Failed to get historical upload data: %v", err)
		historicalData = []database.SystemVitalLog{}
	}

	// Convert to time series points (in MB/s for sparkline)
	var timeSeriesData []charts.TimeSeriesPoint
	for _, metric := range historicalData {
		timeSeriesData = append(timeSeriesData, charts.TimeSeriesPoint{
			Time:  metric.Timestamp,
			Value: float64(metric.UploadRate) / 1024 / 1024, // Convert to MB/s
		})
	}

	// Generate sparkline SVG
	opts := charts.DefaultSparklineOptions()
	opts.ShowNoData = true
	sparklineSVG := charts.GenerateTimeAwareSparkline(timeSeriesData, startTime, now, opts)

	// Prepare data for the template
	data := struct {
		CurrentRate  string
		SparklineSVG template.HTML
	}{
		CurrentRate:  formatNetworkRate(float64(latest.UploadRate)),
		SparklineSVG: sparklineSVG,
	}

	// Get the upload card template
	tmpl, ok := s.templates["_upload_card"]
	if !ok {
		log.Printf("Upload card template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Render the partial template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "upload-card-partial", data); err != nil {
		log.Printf("Error rendering upload card template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// formatNetworkRate formats bytes per second into human-readable format
func formatNetworkRate(bytesPerSecond float64) string {
	if bytesPerSecond < 0 {
		return "0.00 MB/s"
	}

	// Convert bytes per second to megabytes per second
	// 1 MB = 1,048,576 bytes (using binary convention)
	megabytesPerSecond := bytesPerSecond / (1024 * 1024)

	// Always display in MB/s with 2 decimal places
	return fmt.Sprintf("%.2f MB/s", megabytesPerSecond)
}

// normalizeNetworkRates normalizes network rate values to percentages for sparkline display
func normalizeNetworkRates(rates []float64) []float64 {
	if len(rates) == 0 {
		return rates
	}

	// Find the maximum value
	maxRate := rates[0]
	for _, rate := range rates {
		if rate > maxRate {
			maxRate = rate
		}
	}

	// If all values are zero, return as-is
	if maxRate == 0 {
		return rates
	}

	// Normalize to 0-100 scale
	normalized := make([]float64, len(rates))
	for i, rate := range rates {
		normalized[i] = (rate / maxRate) * 100
	}

	return normalized
}
