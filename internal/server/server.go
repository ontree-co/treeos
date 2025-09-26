package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"treeos/internal/cache"
	"treeos/internal/caddy"
	"treeos/internal/charts"
	"treeos/internal/config"
	"treeos/internal/database"
	"treeos/internal/embeds"
	"treeos/internal/ollama"
	"treeos/internal/progress"
	"treeos/internal/realtime"
	containerruntime "treeos/internal/runtime"
	"treeos/internal/system"
	"treeos/internal/templates"
	"treeos/internal/update"
	"treeos/internal/version"
	"treeos/internal/yamlutil"
	"treeos/pkg/compose"
)

// Server represents the HTTP server
type Server struct {
	config                *config.Config
	templates             map[string]*template.Template
	sessionStore          *sessions.CookieStore
	runtimeClient         *containerruntime.Client
	runtimeSvc            *containerruntime.Service
	runtimeMu             sync.Mutex
	runtimeClientHealthy  bool
	runtimeServiceHealthy bool
	db                    *sql.DB
	templateSvc           *templates.Service
	versionInfo           version.Info
	caddyAvailable        bool
	caddyClient           *caddy.Client
	platformSupportsCaddy bool
	sparklineCache        *cache.Cache
	realtimeMetrics       *realtime.Metrics
	composeSvc            *compose.Service
	sseManager            *SSEManager
	ollamaWorker          *ollama.Worker
	progressTracker       *progress.Tracker
	stopCh                chan struct{}
	stopOnce              sync.Once
	updateMu              sync.Mutex
	composeHealthy        bool
}

var (
	errRuntimeUnavailable = errors.New("container runtime not available")
	errComposeUnavailable = errors.New("compose service not available")
)

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
		progressTracker:       progress.NewTracker(),
		stopCh:                make(chan struct{}),
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

	// Initialize database with migration verification
	log.Printf("Initializing database at %s...", cfg.DatabasePath)
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	s.db = db

	// Verify migrations completed by checking a recent column
	// This ensures migrations are fully applied before serving requests
	if err := s.verifyMigrationsComplete(); err != nil {
		return nil, fmt.Errorf("database migrations incomplete: %w", err)
	}
	log.Printf("Database initialized and migrations verified")

	// Initialize container runtime client
	runtimeClient, err := containerruntime.NewClient()
	if err != nil {
		log.Printf("Warning: Failed to initialize container runtime client: %v", err)
		// Continue without container runtime support
	} else {
		s.runtimeClient = runtimeClient
		s.runtimeClientHealthy = true
	}

	// Initialize runtime service
	runtimeSvc, err := containerruntime.NewService(cfg.AppsDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize container runtime service: %v", err)
		// Continue without container runtime support
	} else {
		s.runtimeSvc = runtimeSvc
		s.runtimeServiceHealthy = true
	}

	// Initialize Compose service
	composeSvc, err := compose.NewService()
	if err != nil {
		log.Printf("Warning: Failed to initialize Compose service: %v", err)
		// Continue without Compose support
	} else {
		s.composeSvc = composeSvc
		s.composeHealthy = true
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

	// Initialize SSE manager
	s.sseManager = NewSSEManager()

	// Initialize template service
	templatesPath := "compose" // Path within the embedded templates directory
	s.templateSvc = templates.NewService(templatesPath)

	// Agent will be initialized in Start() if enabled

	return s, nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	s.stopOnce.Do(func() {
		if s.stopCh != nil {
			close(s.stopCh)
		}
	})
	if s.runtimeSvc != nil {
		if err := s.runtimeSvc.Close(); err != nil {
			log.Printf("Error closing container runtime service: %v", err)
		}
	}
	if s.runtimeClient != nil {
		if err := s.runtimeClient.Close(); err != nil {
			log.Printf("Error closing container runtime client: %v", err)
		}
	}
	if s.db != nil {
		// Close the global database connection, not just the local one
		if err := database.Close(); err != nil {
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
	systemCheckTemplate := filepath.Join("templates", "partials", "system_check.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, setupTemplate, systemCheckTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse setup template: %w", err)
	}
	s.templates["setup"] = tmpl

	// Load systemcheck template
	systemCheckPageTemplate := filepath.Join("templates", "dashboard", "systemcheck.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, systemCheckPageTemplate, systemCheckTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse systemcheck template: %w", err)
	}
	s.templates["systemcheck"] = tmpl

	// Load login template
	loginTemplate := filepath.Join("templates", "dashboard", "login.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, loginTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse login template: %w", err)
	}
	s.templates["login"] = tmpl

	// Load settings template
	settingsTemplate := filepath.Join("templates", "dashboard", "settings.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, settingsTemplate, systemCheckTemplate)
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

	// Load model templates list template
	modelTemplatesTemplate := filepath.Join("templates", "dashboard", "model_templates.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, modelTemplatesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse model templates template: %w", err)
	}
	s.templates["model_templates"] = tmpl

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

	// Note: monitoring.html and monitoring_detail.html templates have been removed
	// as monitoring functionality has been integrated into the main dashboard

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

	// Load GPU card template
	gpuCardTemplate := filepath.Join("templates", "dashboard", "_gpu_card.html")
	gpuTmpl, err := embeds.ParseTemplate(gpuCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse gpu card template: %w", err)
	}
	s.templates["_gpu_card"] = gpuTmpl

	// Load Download card template
	downloadCardTemplate := filepath.Join("templates", "dashboard", "_download_card.html")
	downloadTmpl, err := embeds.ParseTemplate(downloadCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse download card template: %w", err)
	}
	s.templates["_download_card"] = downloadTmpl

	// Load Upload card template
	uploadCardTemplate := filepath.Join("templates", "dashboard", "_upload_card.html")
	uploadTmpl, err := embeds.ParseTemplate(uploadCardTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse upload card template: %w", err)
	}
	s.templates["_upload_card"] = uploadTmpl

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

	// Load models list partial template
	modelsListTemplate := filepath.Join("templates", "partials", "models_list.html")
	modelsTmpl, err := embeds.ParseTemplate(modelsListTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse models list template: %w", err)
	}
	s.templates["models_list"] = modelsTmpl

	// Load model detail template
	modelDetailTemplate := filepath.Join("templates", "dashboard", "model_detail.html")
	tmpl, err = embeds.ParseTemplate(baseTemplate, modelDetailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse model detail template: %w", err)
	}
	s.templates["model_detail"] = tmpl

	return nil
}

func (s *Server) getUpdateChannel() update.UpdateChannel {
	if s.db == nil {
		return update.ChannelStable
	}

	var channel string
	err := s.db.QueryRow(`SELECT update_channel FROM system_setup WHERE id = 1`).Scan(&channel)
	if err != nil {
		if err != sql.ErrNoRows && !strings.Contains(err.Error(), "no such column") {
			log.Printf("Failed to get update channel: %v", err)
		}
		return update.ChannelStable
	}

	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "beta":
		return update.ChannelBeta
	default:
		return update.ChannelStable
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start background jobs
	go s.startVitalsCleanup()
	go s.startRealtimeMetricsCollection()
	go s.startVitalsCollection()
	go s.startProgressCleanup()

	// Start Ollama worker if database is available
	if s.db != nil {
		s.startOllamaWorker()
	}
	// No need to schedule them again here

	// Reset update status on startup
	ResetUpdateStatus()

	// Automatic update scheduler
	s.startAutoUpdateScheduler()

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
	mux.HandleFunc("/systemcheck", s.TracingMiddleware(s.SetupRequiredMiddleware(s.handleSetupSystemCheck)))
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
	mux.HandleFunc("/api/models", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeAPIModels))))
	mux.HandleFunc("/api/models/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.routeAPIModels))))
	mux.HandleFunc("/models", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleModelTemplates))))
	mux.HandleFunc("/models/", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleModelDetail))))

	// Test endpoint for checking LLM API connection
	mux.HandleFunc("/api/test-llm", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleTestLLMConnection))))

	// Dashboard partial routes (for monitoring cards on dashboard)
	mux.HandleFunc("/partials/cpu", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringCPUPartial))))
	mux.HandleFunc("/partials/memory", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringMemoryPartial))))
	mux.HandleFunc("/partials/disk", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringDiskPartial))))
	mux.HandleFunc("/partials/network", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringNetworkPartial))))
	mux.HandleFunc("/partials/gpu", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringGPUPartial))))
	mux.HandleFunc("/partials/download", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringDownloadPartial))))
	mux.HandleFunc("/partials/upload", s.TracingMiddleware(s.SetupRequiredMiddleware(s.AuthRequiredMiddleware(s.handleMonitoringUploadPartial))))

	// Version endpoint (no auth required for automation/monitoring)
	mux.HandleFunc("/version", s.TracingMiddleware(s.handleVersion))

	// Logging endpoints
	mux.HandleFunc("/api/log", s.TracingMiddleware(s.handleBrowserLog))
	mux.HandleFunc("/api/logs", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleGetLogs)))

	// System update endpoints
	mux.HandleFunc("/api/system/check", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemCheck)))
	mux.HandleFunc("/api/system/update/check", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemUpdateCheck)))
	mux.HandleFunc("/api/system/update/apply", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemUpdateApply)))
	mux.HandleFunc("/api/system/update/status", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemUpdateStatus)))
	mux.HandleFunc("/api/system/update/channel", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemUpdateChannel)))
	mux.HandleFunc("/api/system/update/history", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemUpdateHistory)))
	mux.HandleFunc("/api/system/update/restart", s.TracingMiddleware(s.AuthRequiredMiddleware(s.handleSystemUpdateRestart)))

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

// startProgressCleanup runs a background job to clean up old progress tracking operations
func (s *Server) startProgressCleanup() {
	log.Printf("Progress tracking cleanup job started")

	// Run cleanup every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Remove operations older than 30 minutes
		s.progressTracker.CleanupOldOperations(30 * time.Minute)
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

func (s *Server) startAutoUpdateScheduler() {
	if !s.config.AutoUpdateEnabled {
		log.Printf("Automatic updates disabled (AUTO_UPDATE_ENABLED=false)")
		return
	}

	go s.autoUpdateLoop()
}

func (s *Server) autoUpdateLoop() {
	log.Printf("Automatic update scheduler started")

	s.runAutoUpdate("startup")

	for {
		next := durationUntilNextUpdate(time.Now())
		timer := time.NewTimer(next)
		select {
		case <-timer.C:
			s.runAutoUpdate("scheduled")
		case <-s.stopCh:
			timer.Stop()
			log.Printf("Automatic update scheduler stopping")
			return
		}
	}
}

func durationUntilNextUpdate(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func (s *Server) runAutoUpdate(trigger string) {
	if !s.config.AutoUpdateEnabled {
		return
	}

	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	channel := s.getUpdateChannel()
	updateSvc := update.NewService(channel)

	info, err := updateSvc.CheckForUpdate()
	if err != nil {
		log.Printf("Auto-update check failed: %v", err)
		return
	}

	status := UpdateStatus{
		CurrentVersion:   info.CurrentVersion,
		AvailableVersion: info.LatestVersion,
		Message:          fmt.Sprintf("Checked for updates (%s channel)", channel),
	}

	if !info.UpdateAvailable {
		SetUpdateStatus(status)
		return
	}

	current := GetUpdateStatus()
	if current.RestartRequired && current.AvailableVersion == info.LatestVersion {
		log.Printf("Update %s already applied and awaiting restart", info.LatestVersion)
		return
	}

	log.Printf("Automatic update found: %s -> %s (trigger=%s)", info.CurrentVersion, info.LatestVersion, trigger)

	started := time.Now()
	SetUpdateStatus(UpdateStatus{
		InProgress:       true,
		Stage:            "downloading",
		Message:          fmt.Sprintf("Downloading update %s", info.LatestVersion),
		CurrentVersion:   info.CurrentVersion,
		AvailableVersion: info.LatestVersion,
		StartedAt:        started,
	})

	err = updateSvc.ApplyUpdate(func(stage string, percentage float64, message string) {
		SetUpdateStatus(UpdateStatus{
			InProgress:       true,
			Stage:            stage,
			Percentage:       percentage,
			Message:          message,
			CurrentVersion:   info.CurrentVersion,
			AvailableVersion: info.LatestVersion,
			StartedAt:        started,
		})
	})

	if err != nil {
		log.Printf("Automatic update failed: %v", err)
		SetUpdateStatus(UpdateStatus{
			Failed:           true,
			Error:            "Automatic update failed. See logs for details.",
			Message:          err.Error(),
			Stage:            "failed",
			CurrentVersion:   info.CurrentVersion,
			AvailableVersion: info.LatestVersion,
		})
		return
	}

	log.Printf("Automatic update to %s applied. Restart required.", info.LatestVersion)
	SetUpdateStatus(UpdateStatus{
		Success:          true,
		RestartRequired:  true,
		Stage:            "complete",
		Percentage:       100,
		Message:          fmt.Sprintf("Update %s installed. Restart required to finish.", info.LatestVersion),
		CurrentVersion:   info.CurrentVersion,
		AvailableVersion: info.LatestVersion,
		StartedAt:        started,
	})

	if s.sseManager != nil {
		s.sseManager.SendToAll("update-ready", map[string]interface{}{
			"version": info.LatestVersion,
		})
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

func (s *Server) getRuntimeClient() (*containerruntime.Client, error) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()

	if s.runtimeClient == nil || !s.runtimeClientHealthy {
		if s.runtimeClient != nil {
			_ = s.runtimeClient.Close()
		}
		client, err := containerruntime.NewClient()
		if err != nil {
			s.runtimeClientHealthy = false
			return nil, fmt.Errorf("%w: %v", errRuntimeUnavailable, err)
		}
		s.runtimeClient = client
		s.runtimeClientHealthy = true
	}

	return s.runtimeClient, nil
}

func (s *Server) getRuntimeService() (*containerruntime.Service, error) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()

	if s.runtimeSvc == nil || !s.runtimeServiceHealthy {
		if s.runtimeSvc != nil {
			_ = s.runtimeSvc.Close()
		}
		svc, err := containerruntime.NewService(s.config.AppsDir)
		if err != nil {
			s.runtimeServiceHealthy = false
			return nil, fmt.Errorf("%w: %v", errRuntimeUnavailable, err)
		}
		s.runtimeSvc = svc
		s.runtimeServiceHealthy = true
	}

	return s.runtimeSvc, nil
}

func (s *Server) getComposeService() (*compose.Service, error) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()

	if s.composeSvc != nil && !s.composeHealthy {
		s.composeHealthy = true
		return s.composeSvc, nil
	}

	if s.composeSvc == nil || !s.composeHealthy {
		if s.composeSvc != nil {
			_ = s.composeSvc.Close()
		}
		svc, err := compose.NewService()
		if err != nil {
			s.composeHealthy = false
			return nil, fmt.Errorf("%w: %v", errComposeUnavailable, err)
		}
		s.composeSvc = svc
		s.composeHealthy = true
	}

	return s.composeSvc, nil
}

func (s *Server) markRuntimeUnhealthy() {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.runtimeClientHealthy = false
	s.runtimeServiceHealthy = false
}

func (s *Server) markComposeUnhealthy() {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.composeSvc != nil {
		_ = s.composeSvc.Close()
		s.composeSvc = nil
	}
	s.composeHealthy = false
}

func isRuntimeUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "podman client not available") {
		return true
	}
	if strings.Contains(msg, "failed to list podman") || strings.Contains(msg, "podman ps failed") {
		return true
	}
	if strings.Contains(msg, "cannot connect") && strings.Contains(msg, "docker") {
		return true
	}
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "permission denied") || strings.Contains(msg, "no such file") {
		return true
	}
	if strings.Contains(msg, "socket") && strings.Contains(msg, "dial") {
		return true
	}
	return false
}

func (s *Server) scanApps() ([]*containerruntime.App, error) {
	client, err := s.getRuntimeClient()
	if err != nil {
		return nil, err
	}

	apps, err := client.ScanApps(s.config.AppsDir)
	if err != nil {
		if isRuntimeUnavailableError(err) {
			s.markRuntimeUnhealthy()
		}
		return nil, err
	}

	return apps, nil
}

func (s *Server) getAppDetails(appName string) (*containerruntime.App, error) {
	client, err := s.getRuntimeClient()
	if err != nil {
		return nil, err
	}

	app, detailErr := client.GetAppDetails(s.config.AppsDir, appName)
	if detailErr != nil && isRuntimeUnavailableError(detailErr) {
		s.markRuntimeUnhealthy()
	}
	return app, detailErr
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
	runtimeApps, err := s.scanApps()
	if err != nil {
		if errors.Is(err, errRuntimeUnavailable) {
			log.Printf("Container runtime not available: %v", err)
		} else {
			log.Printf("Error scanning apps: %v", err)
		}
	} else {
		for _, app := range runtimeApps {
			// Create container info for each service
			type ContainerInfo struct {
				Name   string
				Status string
				State  string
				Uptime string
			}

			// Create an enriched app struct with additional status
			enrichedApp := struct {
				*containerruntime.App
				ServiceCount int
				Containers   []ContainerInfo
			}{
				App: app,
			}

			composeSvc, composeErr := s.getComposeService()
			if composeErr != nil {
				if !errors.Is(composeErr, errComposeUnavailable) {
					log.Printf("Compose service unavailable: %v", composeErr)
				}
			} else {
				appDir := filepath.Join(s.config.AppsDir, app.Name)
				if _, statErr := os.Stat(appDir); statErr == nil {
					ctx := context.Background()
					opts := compose.Options{WorkingDir: appDir}

					containers, psErr := composeSvc.PS(ctx, opts)
					if psErr != nil {
						if isRuntimeUnavailableError(psErr) {
							s.markComposeUnhealthy()
						}
						log.Printf("Failed to get compose status for %s: %v", app.Name, psErr)
					} else if len(containers) > 0 {
						containerInfos := make([]ContainerInfo, 0)
						for _, container := range containers {
							serviceName := extractServiceName(container.Name, app.Name)
							status := mapContainerState(container.State)

							uptime := ""
							if container.State == "running" && container.Status != "" {
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
					} else {
						enrichedApp.ServiceCount = 0
						enrichedApp.Containers = []ContainerInfo{}
					}
				}
			}

			apps = append(apps, enrichedApp)
		}
	}

	// Get node name from database
	db := database.GetDB()
	var nodeName string
	err = db.QueryRow("SELECT node_name FROM system_setup WHERE id = 1").Scan(&nodeName)
	if err != nil || nodeName == "" {
		nodeName = "TreeOS" // Default name
	}

	// Get local IP
	localIP := getLocalIP()

	// Get Tailscale IP
	tailscaleIP := getTailscaleIP()

	// Get latest monitoring data from database
	latest, err := database.GetLatestMetric("")
	if err != nil {
		log.Printf("Failed to get latest metric for dashboard: %v", err)
	}

	// Get historical data for sparklines (last 24 hours)
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	historicalData, err := database.GetMetricsForTimeRange(dayAgo, now)
	if err != nil {
		log.Printf("Failed to get historical metrics for sparklines: %v", err)
	}

	// Generate sparklines for each metric
	var cpuSparkline, memorySparkline, diskSparkline, gpuSparkline, uploadSparkline, downloadSparkline template.HTML
	if len(historicalData) > 1 {
		// Extract data points for each metric
		cpuPoints := make([]float64, len(historicalData))
		memoryPoints := make([]float64, len(historicalData))
		diskPoints := make([]float64, len(historicalData))
		gpuPoints := make([]float64, len(historicalData))
		uploadPoints := make([]float64, len(historicalData))
		downloadPoints := make([]float64, len(historicalData))

		for i, m := range historicalData {
			cpuPoints[i] = m.CPUPercent
			memoryPoints[i] = m.MemoryPercent
			diskPoints[i] = m.DiskUsagePercent
			gpuPoints[i] = m.GPULoad
			uploadPoints[i] = float64(m.UploadRate)
			downloadPoints[i] = float64(m.DownloadRate)
		}

		// Generate SVG sparklines (150x40 pixels to fit in the cards)
		cpuSparkline = charts.GenerateSparklineSVG(cpuPoints, 150, 40)
		memorySparkline = charts.GenerateSparklineSVG(memoryPoints, 150, 40)
		diskSparkline = charts.GenerateSparklineSVG(diskPoints, 150, 40)
		gpuSparkline = charts.GenerateSparklineSVG(gpuPoints, 150, 40)
		// For network rates, normalize the values
		uploadSparkline = charts.GenerateSparklineSVGWithStyle(normalizeNetworkRates(uploadPoints), 150, 40, "#198754", 2)
		downloadSparkline = charts.GenerateSparklineSVGWithStyle(normalizeNetworkRates(downloadPoints), 150, 40, "#198754", 2)
	}

	// Prepare monitoring data with formatting
	var monitoringData map[string]interface{}
	if latest != nil {
		monitoringData = map[string]interface{}{
			"CPUPercent":        fmt.Sprintf("%.1f", latest.CPUPercent),
			"MemoryPercent":     fmt.Sprintf("%.1f", latest.MemoryPercent),
			"DiskUsagePercent":  fmt.Sprintf("%.1f", latest.DiskUsagePercent),
			"GPULoad":           fmt.Sprintf("%.1f", latest.GPULoad),
			"UploadRate":        formatNetworkRate(float64(latest.UploadRate)),
			"DownloadRate":      formatNetworkRate(float64(latest.DownloadRate)),
			"CPUSparkline":      cpuSparkline,
			"MemorySparkline":   memorySparkline,
			"DiskSparkline":     diskSparkline,
			"GPUSparkline":      gpuSparkline,
			"UploadSparkline":   uploadSparkline,
			"DownloadSparkline": downloadSparkline,
		}
	} else {
		// Default values if no data available
		monitoringData = map[string]interface{}{
			"CPUPercent":        "0.0",
			"MemoryPercent":     "0.0",
			"DiskUsagePercent":  "0.0",
			"GPULoad":           "0.0",
			"UploadRate":        "0 B/s",
			"DownloadRate":      "0 B/s",
			"CPUSparkline":      template.HTML(""),
			"MemorySparkline":   template.HTML(""),
			"DiskSparkline":     template.HTML(""),
			"GPUSparkline":      template.HTML(""),
			"UploadSparkline":   template.HTML(""),
			"DownloadSparkline": template.HTML(""),
		}
	}

	// Prepare template data
	data := s.baseTemplateData(user)
	data["Apps"] = apps
	data["AppsDir"] = s.config.AppsDir
	data["Messages"] = nil
	data["CSRFToken"] = ""      // No CSRF yet
	data["Hostname"] = nodeName // Using node name instead of system hostname
	data["LocalIP"] = localIP
	data["TailscaleIP"] = tailscaleIP
	data["MonitoringData"] = monitoringData

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

	// Get node icon and name from database
	db := database.GetDB()
	var nodeIcon, nodeName string
	err := db.QueryRow("SELECT node_icon, node_name FROM system_setup WHERE id = 1").Scan(&nodeIcon, &nodeName)
	if err != nil {
		nodeIcon = "tree1.png" // Default icon
		nodeName = "TreeOS"    // Default name
	}
	if nodeIcon == "" {
		nodeIcon = "tree1.png"
	}
	if nodeName == "" {
		nodeName = "TreeOS"
	}
	data["NodeIcon"] = nodeIcon
	data["NodeName"] = nodeName

	// Update status for header notifications
	status := GetUpdateStatus()
	data["UpdateStatus"] = status
	data["UpdateBadge"] = status.RestartRequired

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
		       agent_llm_api_key,
		       agent_llm_api_url, agent_llm_model,
		       uptime_kuma_base_url
		FROM system_setup
		WHERE id = 1
	`).Scan(&setup.ID, &setup.PublicBaseDomain, &setup.TailscaleAuthKey, &setup.TailscaleTags,
		&setup.AgentLLMAPIKey,
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

	// Update LLM config if not overridden by environment
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
	runtimeSvc, err := s.getRuntimeService()
	if err != nil {
		log.Printf("Container runtime service not available; skipping exposure sync: %v", err)
		return
	}

	apps, err := runtimeSvc.ScanApps()
	if err != nil {
		if isRuntimeUnavailableError(err) {
			s.markRuntimeUnhealthy()
		}
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

		// Use lowercase app name as ID for Caddy route
		appID := strings.ToLower(app.Name)

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
		// Check-update functionality removed - using Visit in Browser instead
		// } else if strings.HasSuffix(path, "/check-update") {
		//	s.handleAppCheckUpdate(w, r)
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
	} else if strings.HasSuffix(path, "/progress/sse") {
		s.handleAPIAppProgressSSE(w, r)
	} else if strings.HasSuffix(path, "/progress") {
		s.handleAPIAppProgress(w, r)
	} else if strings.HasSuffix(path, "/security-bypass") {
		// Toggle security bypass for an app
		s.handleAPIAppSecurityBypass(w, r)
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

// getTailscaleDNS returns the Tailscale DNS name for this machine
func getTailscaleDNS() string {
	// Try to get Tailscale DNS name using the tailscale status command
	cmd := exec.Command("sh", "-c", "tailscale status --json 2>/dev/null | jq -r '.Self.DNSName' 2>/dev/null")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	dnsName := strings.TrimSpace(string(output))
	if dnsName == "" || dnsName == "null" {
		return ""
	}

	return dnsName
}

// verifyMigrationsComplete checks that all expected database columns exist
// This ensures migrations have fully completed before the server starts serving requests
func (s *Server) verifyMigrationsComplete() error {
	// Force a checkpoint first to ensure WAL changes are written to the main database file
	// This is crucial after a self-update when the database file might be in WAL mode
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		// Log but don't fail - not all SQLite builds support WAL
		log.Printf("Warning: Could not checkpoint WAL: %v", err)
	}

	// Check for the most recently added columns to ensure all migrations ran
	// We check these specific columns as they were added in recent updates
	verifyQueries := map[string]string{
		"system_setup.update_channel": "SELECT COUNT(*) FROM pragma_table_info('system_setup') WHERE name='update_channel'",
		"system_setup.node_icon":      "SELECT COUNT(*) FROM pragma_table_info('system_setup') WHERE name='node_icon'",
		"update_history.channel":      "SELECT COUNT(*) FROM pragma_table_info('update_history') WHERE name='channel'",
		"system_vital_logs.gpu_load":  "SELECT COUNT(*) FROM pragma_table_info('system_vital_logs') WHERE name='gpu_load'",
	}

	for description, query := range verifyQueries {
		var colCount int
		row := s.db.QueryRow(query)
		if err := row.Scan(&colCount); err != nil {
			// If we can't even query pragma_table_info, the table might not exist
			return fmt.Errorf("failed to verify %s: %w", description, err)
		}
		if colCount == 0 {
			return fmt.Errorf("migration incomplete: %s does not exist", description)
		}
	}

	// Verify we can actually read from a table with new columns
	var testValue sql.NullString
	err := s.db.QueryRow("SELECT update_channel FROM system_setup LIMIT 1").Scan(&testValue)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("migration verification failed: cannot read update_channel: %w", err)
	}

	return nil
}
