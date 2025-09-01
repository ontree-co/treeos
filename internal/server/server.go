package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/robfig/cron/v3"
	"ontree-node/internal/agent"
	"ontree-node/internal/cache"
	"ontree-node/internal/caddy"
	"ontree-node/internal/config"
	"ontree-node/internal/database"
	"ontree-node/internal/docker"
	"ontree-node/internal/embeds"
	"ontree-node/internal/realtime"
	"ontree-node/internal/system"
	"ontree-node/internal/templates"
	"ontree-node/internal/version"
	"ontree-node/internal/yamlutil"
	"ontree-node/pkg/compose"
)

// Server represents the HTTP server
type Server struct {
	config                *config.Config
	templates             map[string]*template.Template
	sessionStore          *sessions.CookieStore
	dockerClient          *docker.Client
	dockerSvc             *docker.Service
	db                    *sql.DB
	templateSvc           *templates.Service
	versionInfo           version.Info
	caddyAvailable        bool
	caddyClient           *caddy.Client
	platformSupportsCaddy bool
	sparklineCache        *cache.Cache
	realtimeMetrics       *realtime.Metrics
	composeSvc            *compose.Service
	agentOrchestrator     *agent.Orchestrator
	agentCron             *cron.Cron
}

// New creates a new server instance
func New(cfg *config.Config, versionInfo version.Info) (*Server, error) {
	// Create session store with secure key
	// In production, this should be loaded from environment or config
	sessionKey := []byte("your-32-byte-session-key-here!!") // TODO: Load from config

	s := &Server{
		config:                cfg,
		templates:             make(map[string]*template.Template),
		sessionStore:          sessions.NewCookieStore(sessionKey),
		versionInfo:           versionInfo,
		platformSupportsCaddy: runtime.GOOS == "linux",
		sparklineCache:        cache.New(5 * time.Minute), // 5-minute cache for sparklines
		realtimeMetrics:       realtime.NewMetrics(),
	}

	// Configure session store
	s.sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	// Load templates
	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	s.db = db

	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Printf("Warning: Failed to initialize Docker client: %v", err)
		// Continue without Docker support
	} else {
		s.dockerClient = dockerClient
	}

	// Initialize Docker service
	dockerSvc, err := docker.NewService(cfg.AppsDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize Docker service: %v", err)
		// Continue without Docker support
	} else {
		s.dockerSvc = dockerSvc
	}

	// Initialize Compose service
	composeSvc, err := compose.NewService()
	if err != nil {
		log.Printf("Warning: Failed to initialize Compose service: %v", err)
		// Continue without Compose support
	} else {
		s.composeSvc = composeSvc
	}

	// Load configuration from database if not set by environment
	if err := s.loadConfigFromDatabase(); err != nil {
		log.Printf("Warning: Failed to load config from database: %v", err)
	}

	// Initialize Caddy client only on Linux
	if s.platformSupportsCaddy {
		s.caddyClient = caddy.NewClient()
		// Check Caddy availability
		s.checkCaddyHealth()
	} else {
		log.Printf("Caddy integration is not supported on %s platform", runtime.GOOS)
		s.caddyAvailable = false
	}

	// Initialize template service
	templatesPath := "compose" // Path within the embedded templates directory
	s.templateSvc = templates.NewService(templatesPath)

	// Agent will be initialized in Start() if enabled

	return s, nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	s.stopAgent()

	if s.dockerSvc != nil {
		if err := s.dockerSvc.Close(); err != nil {
			log.Printf("Error closing docker service: %v", err)
		}
	}
	if s.dockerClient != nil {
		if err := s.dockerClient.Close(); err != nil {
			log.Printf("Error closing docker client: %v", err)
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
}

// loadTemplates loads all HTML templates
func (s *Server) loadTemplates() error {
	// Load base template
	baseTemplate := filepath.Join("templates", "layouts", "base.html")

	// Load dashboard template
	dashboardTemplate := filepath.Join("templates", "dashboard", "index.html")
	tmpl, err := embeds.ParseTemplate(baseTemplate, dashboardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse dashboard template: %w", err)
	}
	s.templates["dashboard"] = tmpl

	// Load setup template
	setupTemplate := filepath.Join("templates", "dashboard", "setup.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, setupTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse setup template: %w", err)
	}
	s.templates["setup"] = tmpl

	// Load login template
	loginTemplate := filepath.Join("templates", "dashboard", "login.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, loginTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse login template: %w", err)
	}
	s.templates["login"] = tmpl

	// Load settings template
	settingsTemplate := filepath.Join("templates", "dashboard", "settings.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, settingsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse settings template: %w", err)
	}
	s.templates["settings"] = tmpl

	// Load app detail template
	appDetailTemplate := filepath.Join("templates", "dashboard", "app_detail.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appDetailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app detail template: %w", err)
	}
	s.templates["app_detail"] = tmpl

	// Load app create template with emoji picker component
	appCreateTemplate := filepath.Join("templates", "dashboard", "app_create.html")
	emojiPickerTemplate := filepath.Join("templates", "components", "emoji-picker.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appCreateTemplate, emojiPickerTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app create template: %w", err)
	}
	s.templates["app_create"] = tmpl

	// Load app templates list template
	appTemplatesTemplate := filepath.Join("templates", "dashboard", "app_templates.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appTemplatesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app templates template: %w", err)
	}
	s.templates["app_templates"] = tmpl

	// Load app create from template template with emoji picker component
	appCreateFromTemplate := filepath.Join("templates", "dashboard", "app_create_from_template.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appCreateFromTemplate, emojiPickerTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app create from template template: %w", err)
	}
	s.templates["app_create_from_template"] = tmpl

	// Load app compose edit template
	appComposeEditTemplate := filepath.Join("templates", "dashboard", "app_compose_edit.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, appComposeEditTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse app compose edit template: %w", err)
	}
	s.templates["app_compose_edit"] = tmpl

	// Load monitoring template
	monitoringTemplate := filepath.Join("templates", "dashboard", "monitoring.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, monitoringTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse monitoring template: %w", err)
	}
	s.templates["monitoring"] = tmpl

	// Load monitoring partial templates (loaded separately for HTMX updates)
	// Note: These partials don't use the base template since they're HTMX fragments
	cpuCardTemplate := filepath.Join("templates", "dashboard", "_cpu_card.html")
	cpuTmpl, err := embeds.ParseTemplate(cpuCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse cpu card template: %w", err)
	}
	s.templates["_cpu_card"] = cpuTmpl

	memoryCardTemplate := filepath.Join("templates", "dashboard", "_memory_card.html")
	memTmpl, err := embeds.ParseTemplate(memoryCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse memory card template: %w", err)
	}
	s.templates["_memory_card"] = memTmpl

	diskCardTemplate := filepath.Join("templates", "dashboard", "_disk_card.html")
	diskTmpl, err := embeds.ParseTemplate(diskCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse disk card template: %w", err)
	}
	s.templates["_disk_card"] = diskTmpl

	networkCardTemplate := filepath.Join("templates", "dashboard", "_network_card.html")
	netTmpl, err := embeds.ParseTemplate(networkCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse network card template: %w", err)
	}
	s.templates["_network_card"] = netTmpl

	// Load pattern library templates
	// Pattern library index
	patternsIndexTemplate := filepath.Join("templates", "pattern_library", "index.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsIndexTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns index template: %w", err)
	}
	s.templates["patterns_index"] = tmpl

	// Pattern library components
	patternsComponentsTemplate := filepath.Join("templates", "pattern_library", "components.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsComponentsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns components template: %w", err)
	}
	s.templates["patterns_components"] = tmpl

	// Pattern library forms
	patternsFormsTemplate := filepath.Join("templates", "pattern_library", "forms.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsFormsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns forms template: %w", err)
	}
	s.templates["patterns_forms"] = tmpl

	// Pattern library typography
	patternsTypographyTemplate := filepath.Join("templates", "pattern_library", "typography.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsTypographyTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns typography template: %w", err)
	}
	s.templates["patterns_typography"] = tmpl

	// Pattern library partials
	patternsPartialsTemplate := filepath.Join("templates", "pattern_library", "partials.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsPartialsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns partials template: %w", err)
	}
	s.templates["patterns_partials"] = tmpl

	// Pattern library layouts
	patternsLayoutsTemplate := filepath.Join("templates", "pattern_library", "layouts.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsLayoutsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns layouts template: %w", err)
	}
	s.templates["patterns_layouts"] = tmpl

	// Pattern library style guide
	patternsStyleGuideTemplate := filepath.Join("templates", "pattern_library", "style_guide.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, patternsStyleGuideTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse patterns style guide template: %w", err)
	}
	s.templates["patterns_style_guide"] = tmpl

	return nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start background jobs
	go s.startVitalsCleanup()
	go s.startRealtimeMetricsCollection()
	go s.startVitalsCollection()

	// Start agent cron job if configured
	if s.agentOrchestrator != nil && s.agentCron != nil {
		// Schedule the periodic check
		checkInterval := s.config.AgentCheckInterval
		cronSpec := fmt.Sprintf("@every %s", checkInterval)

		_, err := s.agentCron.AddFunc(cronSpec, func() {
			ctx := context.Background()
			if err := s.agentOrchestrator.RunCheck(ctx); err != nil {
				log.Printf("Agent check failed: %v", err)
			}
		})

		if err != nil {
			log.Printf("Warning: Failed to schedule agent cron job: %v", err)
		} else {
			// Start the cron scheduler
			s.agentCron.Start()
			log.Printf("Agent cron job started with interval: %s", checkInterval)

			// Run initial check
			go func() {
				ctx := context.Background()
				log.Println("Running initial agent check...")
				if err := s.agentOrchestrator.RunCheck(ctx); err != nil {
					log.Printf("Initial agent check failed: %v", err)
				}
			}()
		}
	}

	// Set up routes
	mux := http.NewServeMux()

	// Static file serving using embedded files
	staticFS, err := embeds.StaticFS()
	if err != nil {
		return fmt.Errorf("failed to get static filesystem: %w", err)
	}
	fs := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Public routes (no auth required)
	mux.HandleFunc("/setup", s.TracingMiddleware(s.SetupRequiredMiddleware(s.handleSetup)))
	mux.HandleFunc("/login", s.TracingMiddleware(s.SetupRequiredMiddleware(s.handleLogin)))
	mux.HandleFunc("/logout", s.TracingMiddleware(s.handleLogout))

	// Protected routes (auth required)
	mux.HandleFunc("/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleDashboard))))
	mux.HandleFunc("/apps/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeApps))))
	mux.HandleFunc("/templates", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleTemplates))))
	mux.HandleFunc("/templates/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeTemplates))))

	// API routes
	mux.HandleFunc("/api/apps/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeAPIApps))))
	mux.HandleFunc("/api/v1/status/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeAPIStatus))))

	// Test endpoint for triggering agent runs (for testing purposes)
	// This endpoint is protected by auth middleware so only authenticated users can trigger it
	mux.HandleFunc("/api/test/agent-run", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleTestAgentRun))))
	
	// Test endpoint for checking LLM API connection
	mux.HandleFunc("/api/test-agent", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleTestAgentConnection))))

	// Dashboard partial routes (for monitoring cards on dashboard)
	mux.HandleFunc("/partials/cpu", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringCPUPartial))))
	mux.HandleFunc("/partials/memory", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringMemoryPartial))))
	mux.HandleFunc("/partials/disk", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringDiskPartial))))
	mux.HandleFunc("/partials/network", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringNetworkPartial))))

	// Version endpoint (no auth required for automation/monitoring)
	mux.HandleFunc("/version", s.TracingMiddleware(s.handleVersion))

	// Pattern library routes (no auth required - public access)
	mux.HandleFunc("/patterns", s.TracingMiddleware(s.routePatterns))
	mux.HandleFunc("/patterns/", s.TracingMiddleware(s.routePatterns))

	// Component routes (no auth required - public access for HTMX components)
	mux.HandleFunc("/components/", s.TracingMiddleware(s.routeComponents))

	// Settings routes
	mux.HandleFunc("/settings", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			s.handleSettingsUpdate(w, r)
		} else {
			s.handleSettings(w, r)
		}
	}))))

	// Monitoring routes (only if enabled)
	if s.config.MonitoringEnabled {
		mux.HandleFunc("/monitoring", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoring))))
		mux.HandleFunc("/monitoring/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeMonitoring))))
	}

	// Start agent if configured
	if s.config.AgentEnabled {
		if err := s.restartAgent(); err != nil {
			log.Printf("Failed to start agent: %v", err)
		}
	}

	// Start server
	addr := s.config.ListenAddr
	if addr == "" {
		addr = config.DefaultPort
	}

	log.Printf("Starting server on %s", addr)

	// Create server with proper timeouts
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// startVitalsCleanup runs a background job to clean up old system vital logs
func (s *Server) startVitalsCleanup() {
	log.Printf("System vitals cleanup job started")

	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup on startup
	s.cleanupOldVitals()

	for range ticker.C {
		s.cleanupOldVitals()
	}
}

// cleanupOldVitals removes system vital logs older than 7 days
func (s *Server) cleanupOldVitals() {
	db := database.GetDB()

	// Delete records older than 7 days
	query := `
		DELETE FROM system_vital_logs 
		WHERE timestamp < datetime('now', '-7 days')
	`

	result, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to cleanup old vitals: %v", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get rows affected: %v", err)
		return
	}

	if rowsAffected > 0 {
		log.Printf("Cleaned up %d old vital log records", rowsAffected)
	}
}

// startVitalsCollection periodically collects and stores system vitals to the database
func (s *Server) startVitalsCollection() {
	log.Printf("System vitals collection started (storing to database every 30 seconds)")

	// Collect and store vitals every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Store initial vitals on startup
	s.storeVitals()

	for range ticker.C {
		s.storeVitals()
	}
}

// storeVitals collects current system vitals and stores them to the database
func (s *Server) storeVitals() {
	vitals, err := system.GetVitals()
	if err != nil {
		log.Printf("Failed to get system vitals for storage: %v", err)
		return
	}

	err = database.StoreSystemVital(
		vitals.CPUPercent,
		vitals.MemPercent,
		vitals.DiskPercent,
		vitals.GPULoad,
		vitals.UploadRate,
		vitals.DownloadRate,
	)
	if err != nil {
		log.Printf("Failed to store system vitals: %v", err)
		return
	}

	// Log successful storage for debugging (can be removed in production)
	log.Printf("Stored system vitals: CPU=%.1f%%, Mem=%.1f%%, Disk=%.1f%%, GPU=%.1f%%, Upload=%d B/s, Download=%d B/s",
		vitals.CPUPercent, vitals.MemPercent, vitals.DiskPercent, vitals.GPULoad,
		vitals.UploadRate, vitals.DownloadRate)
}

// startRealtimeMetricsCollection collects CPU and network metrics every second for real-time display
func (s *Server) startRealtimeMetricsCollection() {
	log.Printf("Real-time metrics collection started")

	// Run collection every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Get current system vitals
		vitals, err := system.GetVitals()
		if err != nil {
			log.Printf("Failed to collect real-time metrics: %v", err)
			continue
		}

		// Store CPU metric
		s.realtimeMetrics.AddCPU(vitals.CPUPercent)

		// Store network metrics using cumulative counters for realtime rate calculation
		rxBytes, txBytes := system.GetNetworkCounters()
		s.realtimeMetrics.AddNetwork(rxBytes, txBytes)
	}
}

// handleDashboard handles the dashboard page
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Only handle exact path match
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get user from context
	user := getUserFromContext(r.Context())

	// Scan for applications
	var apps []interface{}
	if s.dockerClient != nil {
		dockerApps, err := s.dockerClient.ScanApps(s.config.AppsDir)
		if err != nil {
			log.Printf("Error scanning apps: %v", err)
		} else {
			// For each app, enrich with additional status information
			for _, app := range dockerApps {
				// Create container info for each service
				type ContainerInfo struct {
					Name   string
					Status string
					State  string
					Uptime string
				}

				// Create an enriched app struct with additional status
				enrichedApp := struct {
					*docker.App
					ServiceCount int
					Containers   []ContainerInfo
				}{
					App: app,
				}

				// Get container status for the app
				if s.composeSvc != nil {
					// Use internal call to get status
					appDir := filepath.Join(s.config.AppsDir, app.Name)
					if _, err := os.Stat(appDir); err == nil {
						ctx := context.Background()
						opts := compose.Options{
							WorkingDir: appDir,
						}

						containers, err := s.composeSvc.PS(ctx, opts)
						if err == nil && len(containers) > 0 {
							// Calculate service count and aggregate status
							containerInfos := make([]ContainerInfo, 0)
							for _, container := range containers {
								serviceName := extractServiceName(container.Name, app.Name)
								status := mapContainerState(container.State)

								// Extract uptime from Status field
								uptime := ""
								if container.State == "running" && container.Status != "" {
									// Status typically contains "Up 2 hours" or similar
									uptime = container.Status
								}

								containerInfos = append(containerInfos, ContainerInfo{
									Name:   serviceName,
									Status: status,
									State:  container.State,
									Uptime: uptime,
								})
							}

							enrichedApp.ServiceCount = len(containerInfos)
							enrichedApp.Containers = containerInfos
							// Status is already in app.Status from docker.ScanApps
						} else {
							// No containers or error
							enrichedApp.ServiceCount = 0
							enrichedApp.Containers = []ContainerInfo{}
						}
					}
				}

				apps = append(apps, enrichedApp)
			}
		}
	}

	// Get system information
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "Unknown"
	}

	// Get local IP
	localIP := getLocalIP()

	// Get Tailscale IP
	tailscaleIP := getTailscaleIP()

	// Prepare template data
	data := s.baseTemplateData(user)
	data["Apps"] = apps
	data["AppsDir"] = s.config.AppsDir
	data["Messages"] = nil
	data["CSRFToken"] = "" // No CSRF yet
	data["Hostname"] = hostname
	data["LocalIP"] = localIP
	data["TailscaleIP"] = tailscaleIP

	// Render template
	tmpl, ok := s.templates["dashboard"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
}

// getUserInitial gets the first letter of username in uppercase
func getUserInitial(username string) string {
	if username == "" {
		return "?"
	}
	return strings.ToUpper(string(username[0]))
}

// baseTemplateData creates the common data structure for base template
func (s *Server) baseTemplateData(user *database.User) map[string]interface{} {
	data := make(map[string]interface{})

	if user != nil {
		data["User"] = user
		data["UserInitial"] = getUserInitial(user.Username)

		// PostHog configuration
		if s.config.PostHogAPIKey != "" {
			data["PostHogEnabled"] = true
			data["PostHogAPIKey"] = s.config.PostHogAPIKey
			data["PostHogHost"] = s.config.PostHogHost
		}
	}

	// Version info
	data["Version"] = s.versionInfo.Version
	data["VersionAge"] = version.GetVersionAge()

	// Caddy availability
	data["CaddyAvailable"] = s.caddyAvailable
	data["PlatformSupportsCaddy"] = s.platformSupportsCaddy

	// Monitoring availability
	data["MonitoringEnabled"] = s.config.MonitoringEnabled

	// Messages field is required by base template
	data["Messages"] = nil

	return data
}

// checkCaddyHealth checks if Caddy is available and running
func (s *Server) checkCaddyHealth() {
	// Skip Caddy checks on non-Linux platforms
	if !s.platformSupportsCaddy {
		s.caddyAvailable = false
		return
	}

	if s.caddyClient == nil {
		s.caddyAvailable = false
		return
	}

	err := s.caddyClient.HealthCheck()
	if err != nil {
		log.Printf("Cannot connect to Caddy Admin API at localhost:2019. Please ensure Caddy is installed and running. Error: %v", err)
		s.caddyAvailable = false
		return
	}

	log.Printf("Successfully connected to Caddy Admin API")
	s.caddyAvailable = true

	// Sync exposed apps if database is available
	if s.db != nil && s.caddyAvailable {
		s.syncExposedApps()
	}
}

// loadConfigFromDatabase loads configuration from database if not set by environment
func (s *Server) loadConfigFromDatabase() error {
	// Query database for all configuration
	var setup database.SystemSetup
	err := s.db.QueryRow(`
		SELECT id, public_base_domain, tailscale_auth_key, tailscale_tags,
		       agent_enabled, agent_check_interval, agent_llm_api_key,
		       agent_llm_api_url, agent_llm_model,
		       uptime_kuma_base_url
		FROM system_setup 
		WHERE id = 1
	`).Scan(&setup.ID, &setup.PublicBaseDomain, &setup.TailscaleAuthKey, &setup.TailscaleTags,
		&setup.AgentEnabled, &setup.AgentCheckInterval, &setup.AgentLLMAPIKey,
		&setup.AgentLLMAPIURL, &setup.AgentLLMModel,
		&setup.UptimeKumaBaseURL)

	if err != nil {
		if err == sql.ErrNoRows {
			// No config yet, that's OK
			return nil
		}
		return fmt.Errorf("failed to query config: %w", err)
	}

	// Update domain config if not overridden by environment
	if os.Getenv("PUBLIC_BASE_DOMAIN") == "" && setup.PublicBaseDomain.Valid {
		s.config.PublicBaseDomain = setup.PublicBaseDomain.String
	}
	if os.Getenv("TAILSCALE_AUTH_KEY") == "" && setup.TailscaleAuthKey.Valid {
		s.config.TailscaleAuthKey = setup.TailscaleAuthKey.String
	}
	if os.Getenv("TAILSCALE_TAGS") == "" && setup.TailscaleTags.Valid {
		s.config.TailscaleTags = setup.TailscaleTags.String
	}

	// Update agent config if not overridden by environment
	if os.Getenv("AGENT_ENABLED") == "" && setup.AgentEnabled.Valid {
		s.config.AgentEnabled = setup.AgentEnabled.Int64 == 1
	}
	if os.Getenv("AGENT_CHECK_INTERVAL") == "" && setup.AgentCheckInterval.Valid {
		s.config.AgentCheckInterval = setup.AgentCheckInterval.String
	}
	if os.Getenv("AGENT_LLM_API_KEY") == "" && setup.AgentLLMAPIKey.Valid {
		s.config.AgentLLMAPIKey = setup.AgentLLMAPIKey.String
	}
	if os.Getenv("AGENT_LLM_API_URL") == "" && setup.AgentLLMAPIURL.Valid {
		s.config.AgentLLMAPIURL = setup.AgentLLMAPIURL.String
	}
	if os.Getenv("AGENT_LLM_MODEL") == "" && setup.AgentLLMModel.Valid {
		s.config.AgentLLMModel = setup.AgentLLMModel.String
	}
	if os.Getenv("UPTIME_KUMA_BASE_URL") == "" && setup.UptimeKumaBaseURL.Valid {
		s.config.UptimeKumaBaseURL = setup.UptimeKumaBaseURL.String
	}

	return nil
}

// stopAgent stops the agent cron scheduler and orchestrator
func (s *Server) stopAgent() {
	// Stop agent cron if running
	if s.agentCron != nil {
		log.Println("Stopping agent cron scheduler...")
		ctx := s.agentCron.Stop()
		<-ctx.Done()
		s.agentCron = nil
	}

	// Close agent orchestrator
	if s.agentOrchestrator != nil {
		if err := s.agentOrchestrator.Close(); err != nil {
			log.Printf("Error closing agent orchestrator: %v", err)
		}
		s.agentOrchestrator = nil
	}
}

// restartAgent restarts the agent with updated configuration
func (s *Server) restartAgent() error {
	// Stop existing agent
	s.stopAgent()

	// Check if agent should be enabled
	if !s.config.AgentEnabled {
		log.Println("Agent is disabled")
		return nil
	}

	if s.config.AgentLLMAPIKey == "" {
		log.Printf("Warning: Agent is enabled but AGENT_LLM_API_KEY is not set. Agent will run in fallback mode.")
	}

	// Parse check interval
	checkInterval, err := time.ParseDuration(s.config.AgentCheckInterval)
	if err != nil {
		log.Printf("Warning: Invalid agent check interval '%s', using default 5m: %v", s.config.AgentCheckInterval, err)
		checkInterval = 5 * time.Minute
	}

	orchestratorConfig := agent.OrchestratorConfig{
		ConfigRootDir:     s.config.AppsDir, // Use the apps directory directly
		UptimeKumaBaseURL: s.config.UptimeKumaBaseURL,
		LLMConfig: agent.LLMConfig{
			APIKey: s.config.AgentLLMAPIKey,
			APIURL: s.config.AgentLLMAPIURL,
			Model:  s.config.AgentLLMModel,
		},
		CheckInterval: checkInterval,
	}

	orchestrator, err := agent.NewOrchestrator(orchestratorConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize agent orchestrator: %w", err)
	}

	s.agentOrchestrator = orchestrator

	// Initialize and start cron scheduler
	s.agentCron = cron.New(cron.WithSeconds())

	// Schedule the agent check
	_, err = s.agentCron.AddFunc(fmt.Sprintf("@every %s", checkInterval), func() {
		ctx := context.Background()
		if err := s.agentOrchestrator.RunCheck(ctx); err != nil {
			log.Printf("ERROR: Agent check failed: %v", err)
		}
	})
	if err != nil {
		s.agentOrchestrator.Close()
		s.agentOrchestrator = nil
		s.agentCron = nil
		return fmt.Errorf("failed to schedule agent cron job: %w", err)
	}

	// Start the cron scheduler
	s.agentCron.Start()

	// Run initial check
	go func() {
		ctx := context.Background()
		if err := s.agentOrchestrator.RunCheck(ctx); err != nil {
			log.Printf("ERROR: Initial agent check failed: %v", err)
		}
	}()

	log.Printf("Agent orchestrator started with check interval: %v", checkInterval)
	return nil
}

// testLLMConnection tests the LLM API connection with a simple ping message
func (s *Server) testLLMConnection(apiKey, apiURL, model string) (string, error) {
	// Create a simple test message
	requestBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Respond with exactly the word: pong",
			},
		},
		"max_completion_tokens": 200, // Increased significantly for reasoning models
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Make the request with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Try to parse error message
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, errorResp.Error.Message)
		}
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the API response
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	response := apiResponse.Choices[0].Message.Content
	
	// Handle empty response gracefully
	if response == "" {
		return "Connection successful! (Empty response from model)", nil
	}
	
	return response, nil
}

// syncExposedApps synchronizes exposed apps with Caddy on startup
func (s *Server) syncExposedApps() {
	// Read all apps from the apps directory
	apps, err := s.dockerSvc.ScanApps()
	if err != nil {
		log.Printf("Failed to list apps: %v", err)
		return
	}

	// Get base domains from config
	publicDomain := s.config.PublicBaseDomain

	for _, app := range apps {
		// Read metadata from compose file
		metadata, err := yamlutil.ReadComposeMetadata(app.Path)
		if err != nil {
			log.Printf("Failed to read metadata for app %s: %v", app.Name, err)
			continue
		}

		// Skip if not exposed
		if metadata == nil || !metadata.IsExposed || metadata.Subdomain == "" || metadata.HostPort == 0 {
			continue
		}

		// Generate ID for Caddy route
		appID := fmt.Sprintf("app-%s", app.Name)

		// Create route config (only for public domain now, Tailscale handled separately)
		routeConfig := caddy.CreateRouteConfig(appID, metadata.Subdomain, metadata.HostPort, publicDomain, "")

		// Add route to Caddy
		err = s.caddyClient.AddOrUpdateRoute(routeConfig)
		if err != nil {
			log.Printf("Failed to sync app %s to Caddy: %v", app.Name, err)
		} else {
			log.Printf("Successfully synced app %s to Caddy", app.Name)
		}
	}
}

// routeApps routes all /apps/* requests to the appropriate handler
func (s *Server) routeApps(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Debug logging
	if strings.Contains(path, "expose") {
		log.Printf("[routeApps] Request: method=%s path=%s", r.Method, path)
	}

	// Route based on the path pattern
	if path == "/apps/create" {
		s.handleAppCreate(w, r)
	} else if strings.HasSuffix(path, "/expose-tailscale") {
		s.handleAppExposeTailscale(w, r)
	} else if strings.HasSuffix(path, "/unexpose-tailscale") {
		s.handleAppUnexposeTailscale(w, r)
	} else if strings.HasSuffix(path, "/expose") {
		s.handleAppExpose(w, r)
	} else if strings.HasSuffix(path, "/unexpose") {
		s.handleAppUnexpose(w, r)
	} else if strings.HasSuffix(path, "/edit") {
		if r.Method == "POST" {
			s.handleAppComposeUpdate(w, r)
		} else {
			s.handleAppComposeEdit(w, r)
		}
	} else if strings.HasSuffix(path, "/containers") {
		s.handleAppContainers(w, r)
	} else if strings.HasSuffix(path, "/check-update") {
		s.handleAppCheckUpdate(w, r)
	} else if strings.HasSuffix(path, "/update") {
		s.handleAppUpdate(w, r)
	} else {
		// Default to app detail page
		s.handleAppDetail(w, r)
	}
}

// routeAPIApps routes /api/apps/* requests
func (s *Server) routeAPIApps(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	if path == "/api/apps" || path == "/api/apps/" {
		// Handle app creation
		s.handleCreateApp(w, r)
	} else if strings.HasSuffix(path, "/status") {
		// Route to different handlers based on content type
		if r.Header.Get("Accept") == "application/json" || r.Method == http.MethodGet {
			// Use the new API status handler for JSON responses
			s.handleAPIAppStatus(w, r)
		} else {
			// Keep the old handler for HTML responses (subdomain checks)
			s.handleAppStatusCheck(w, r)
		}
	} else if strings.HasSuffix(path, "/start") {
		s.handleAPIAppStart(w, r)
	} else if strings.HasSuffix(path, "/stop") {
		s.handleAPIAppStop(w, r)
	} else if strings.HasSuffix(path, "/logs") {
		s.handleAPIAppLogs(w, r)
	} else if strings.HasSuffix(path, "/chat") {
		s.handleAPIAppChat(w, r)
	} else if strings.HasPrefix(path, "/api/apps/") {
		// Check if it's a DELETE request for app deletion
		if r.Method == http.MethodDelete {
			s.handleAPIAppDelete(w, r)
		} else if r.Method == http.MethodGet {
			// Handle GET request to fetch app configuration
			s.handleGetApp(w, r)
		} else {
			// Handle app updates - extract app name and route
			s.handleUpdateApp(w, r)
		}
	} else {
		http.NotFound(w, r)
	}
}

// routeAPIStatus routes /api/v1/status/* requests
func (s *Server) routeAPIStatus(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	switch path {
	case "/api/v1/status/latest", "/api/v1/status/latest/":
		s.handleAPIStatusLatest(w, r)
	case "/api/v1/status/history", "/api/v1/status/history/":
		s.handleAPIStatusHistory(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleVersion returns version information as JSON
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	versionData := map[string]interface{}{
		"version":   s.versionInfo.Version,
		"commit":    s.versionInfo.Commit,
		"buildDate": s.versionInfo.BuildDate,
		"goVersion": s.versionInfo.GoVersion,
		"compiler":  s.versionInfo.Compiler,
		"platform":  s.versionInfo.Platform,
	}

	if err := json.NewEncoder(w).Encode(versionData); err != nil {
		log.Printf("Error encoding version response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getLocalIP returns the primary local IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "Unknown"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				// Skip Docker bridge networks and Tailscale IP
				if !strings.HasPrefix(ip, "172.") && !strings.HasPrefix(ip, "100.") {
					return ip
				}
			}
		}
	}

	return "Unknown"
}

// getTailscaleIP returns the Tailscale IP address if available
func getTailscaleIP() string {
	// Try to get Tailscale IP using the tailscale command
	cmd := exec.Command("tailscale", "ip", "-4")
	output, err := cmd.Output()
	if err != nil {
		// If tailscale command fails, try to find 100.x.x.x IP from interfaces
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return "Not connected"
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip := ipnet.IP.String()
					if strings.HasPrefix(ip, "100.") {
						return ip
					}
				}
			}
		}
		return "Not connected"
	}

	return strings.TrimSpace(string(output))
}
