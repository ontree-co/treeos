package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"ontree-node/internal/database"
)

// Orchestrator coordinates the agent's check-analyze-act cycle
type Orchestrator struct {
	configProvider   ConfigProvider
	collector        *Collector
	reasoningService *ReasoningService
	dockerClient     *client.Client
	checkInterval    time.Duration
	setupHandler     *InitialSetupHandler
	appsDir          string
}

// OrchestratorConfig contains configuration for the Orchestrator
type OrchestratorConfig struct {
	ConfigRootDir     string
	UptimeKumaBaseURL string
	LLMConfig         LLMConfig
	CheckInterval     time.Duration
}

// NewOrchestrator creates a new Orchestrator instance
func NewOrchestrator(config OrchestratorConfig) (*Orchestrator, error) {
	// Create config provider
	configProvider := NewFilesystemProvider(config.ConfigRootDir)

	// Create collector
	collector, err := NewCollector(config.UptimeKumaBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create collector: %w", err)
	}

	// Create reasoning service
	reasoningService, err := NewReasoningService(config.LLMConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create reasoning service: %w", err)
	}

	// Create Docker client for action execution
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Default check interval if not specified
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Minute
	}

	// Create initial setup handler
	setupHandler := NewInitialSetupHandler(config.ConfigRootDir)

	return &Orchestrator{
		configProvider:   configProvider,
		collector:        collector,
		reasoningService: reasoningService,
		dockerClient:     dockerClient,
		checkInterval:    config.CheckInterval,
		setupHandler:     setupHandler,
		appsDir:          config.ConfigRootDir,
	}, nil
}

// GetAllConfigs returns all app configurations
func (o *Orchestrator) GetAllConfigs() ([]AppConfig, error) {
	return o.configProvider.GetAll()
}

// Close cleans up resources
func (o *Orchestrator) Close() error {
	if o.collector != nil {
		if err := o.collector.Close(); err != nil {
			return fmt.Errorf("failed to close collector: %w", err)
		}
	}
	if o.dockerClient != nil {
		if err := o.dockerClient.Close(); err != nil {
			return fmt.Errorf("failed to close Docker client: %w", err)
		}
	}
	return nil
}

// RunCheck performs a check cycle for all configured apps
// This is now mainly for backward compatibility and the initial check
func (o *Orchestrator) RunCheck(ctx context.Context) error {
	log.Println("Starting agent check cycle for all apps...")

	// Step 1: Get all app configurations
	configs, err := o.configProvider.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get app configurations: %w", err)
	}

	if len(configs) == 0 {
		log.Println("No applications configured, skipping check")
		return nil
	}

	log.Printf("Found %d configured applications", len(configs))

	// Run individual checks for each app
	var lastErr error
	for _, config := range configs {
		if err := o.RunCheckForApp(ctx, config.ID); err != nil {
			log.Printf("Error checking app %s: %v", config.ID, err)
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("some app checks failed: %w", lastErr)
	}

	log.Println("Agent check cycle completed for all apps")
	return nil
}

// RunCheckForApp performs a single check-analyze-act cycle for a specific app
func (o *Orchestrator) RunCheckForApp(ctx context.Context, appID string) error {
	log.Printf("Starting agent check for app: %s", appID)

	// Step 1: Get the specific app configuration
	config, err := o.configProvider.GetByID(appID)
	if err != nil {
		return fmt.Errorf("failed to get app configuration for %s: %w", appID, err)
	}

	// Step 1.5: Check if app requires initial setup
	if config.InitialSetupRequired {
		log.Printf("App %s requires initial setup, handling setup first...", config.Name)

		// Create a progress channel for logging
		progressChan := make(chan SetupProgress, 100)
		go func() {
			for progress := range progressChan {
				if progress.IsError {
					log.Printf("Setup ERROR [%d/%d] %s: %s",
						progress.Step, progress.TotalSteps, progress.StepName, progress.Message)
					// Persist error to chat
					o.persistSetupMessage(config.Name, fmt.Sprintf("❌ Initial Setup Failed: %s", progress.Message), "ERROR")
				} else {
					log.Printf("Setup [%d/%d] %s: %s",
						progress.Step, progress.TotalSteps, progress.StepName, progress.Message)

					// Create a user-friendly message based on the step
					var chatMessage string
					switch progress.Step {
					case 1:
						chatMessage = "🔍 Starting initial setup - detecting Docker images..."
					case 2:
						chatMessage = "📦 Fetching latest version information..."
					case 3:
						chatMessage = "🔄 Updating configuration with version locks..."
					case 4:
						chatMessage = "⬇️ Pulling Docker images (this may take a few minutes)..."
					case 5:
						chatMessage = "🚀 Starting containers..."
					case 6:
						if strings.Contains(progress.Message, "completed successfully") {
							chatMessage = "✅ Initial setup completed! Application is ready to use."
						} else {
							chatMessage = "📝 Finalizing setup..."
						}
					default:
						chatMessage = fmt.Sprintf("Step %d/%d: %s", progress.Step, progress.TotalSteps, progress.StepName)
					}

					// Persist progress to chat
					o.persistSetupMessage(config.Name, chatMessage, "INFO")
				}
			}
		}()

		// Handle initial setup
		if err := o.setupHandler.HandleInitialSetup(ctx, config, progressChan); err != nil {
			log.Printf("ERROR: Initial setup failed for %s: %v", config.Name, err)
			close(progressChan)
			return fmt.Errorf("initial setup failed: %w", err)
		}
		log.Printf("Initial setup completed successfully for %s", config.Name)
		close(progressChan)
		return nil // Initial setup completed, no regular check needed
	}

	// Step 2: Collect system snapshot for this specific app
	snapshot, err := o.collector.CollectSystemSnapshotForApp(config)
	if err != nil {
		return fmt.Errorf("failed to collect system snapshot for app %s: %w", appID, err)
	}

	log.Printf("Collected snapshot for app %s", appID)

	// Step 3: Analyze with LLM
	log.Printf("Starting LLM analysis for app %s...", appID)
	llmResponse, err := o.reasoningService.AnalyzeSnapshot(ctx, snapshot)
	if err != nil {
		// Log error but don't fail the entire check
		log.Printf("WARNING: Failed to analyze snapshot with LLM for app %s: %v", appID, err)
		// Create a fallback response
		llmResponse = o.createFallbackResponse(snapshot, []AppConfig{config})
	}

	log.Printf("LLM analysis complete for app %s: status=%s, %d actions recommended",
		appID, llmResponse.OverallStatus, len(llmResponse.RecommendedActions))

	// Step 4: Execute recommended actions
	if err := o.executeActions(ctx, llmResponse.RecommendedActions); err != nil {
		// Log error but continue - some actions may have succeeded
		log.Printf("WARNING: Some actions failed to execute for app %s: %v", appID, err)
	}

	log.Printf("Agent check completed for app: %s", appID)
	return nil
}

// executeActions executes the recommended actions from the LLM
func (o *Orchestrator) executeActions(ctx context.Context, actions []RecommendedAction) error {
	var lastErr error

	for i, action := range actions {
		log.Printf("Executing action %d: %s", i+1, action.ActionKey)

		switch action.ActionKey {
		case ActionPersistChatMessage:
			if err := o.executePersistChatMessage(action); err != nil {
				log.Printf("ERROR: Failed to persist chat message: %v", err)
				lastErr = err
			}

		case ActionRestartContainer:
			if err := o.executeRestartContainer(ctx, action); err != nil {
				log.Printf("ERROR: Failed to restart container: %v", err)
				lastErr = err
			}

		case ActionNoAction:
			log.Println("No action required")

		default:
			log.Printf("WARNING: Unknown action key: %s", action.ActionKey)
		}
	}

	return lastErr
}

// executePersistChatMessage persists a chat message to the database
func (o *Orchestrator) executePersistChatMessage(action RecommendedAction) error {
	// Extract parameters
	appID, ok := action.Parameters["app_id"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid app_id parameter")
	}

	status, ok := action.Parameters["status"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid status parameter")
	}

	message, ok := action.Parameters["message"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid message parameter")
	}

	// Map status to new status levels
	var statusLevel string
	switch strings.ToUpper(status) {
	case "ALL_OK", "OK", "GOOD", "HEALTHY":
		statusLevel = database.StatusLevelInfo
	case "WARNING", "WARN":
		statusLevel = database.StatusLevelWarning
	case "ERROR", "FAIL":
		statusLevel = database.StatusLevelError
	case "CRITICAL":
		statusLevel = database.StatusLevelCritical
	default:
		// Default to warning for unknown statuses
		log.Printf("Unknown status '%s', defaulting to warning", status)
		statusLevel = database.StatusLevelWarning
	}

	// For now, we'll use default values for model/provider
	// TODO: Add methods to ReasoningService to expose model and provider info
	agentModel := "gpt-4"
	agentProvider := database.ProviderOpenAI

	// Create the chat message with new schema
	chatMessage := database.ChatMessage{
		AppID:         appID,
		Timestamp:     time.Now(),
		Message:       message,
		SenderType:    database.SenderTypeAgent,
		SenderName:    "Monitoring Agent",
		AgentModel:    sql.NullString{String: agentModel, Valid: agentModel != ""},
		AgentProvider: sql.NullString{String: agentProvider, Valid: agentProvider != ""},
		StatusLevel:   sql.NullString{String: statusLevel, Valid: true},
	}

	// If there's additional detail, add it
	if action.Justification != "" {
		chatMessage.Details = sql.NullString{String: action.Justification, Valid: true}
	}

	// Save to database
	if err := database.CreateChatMessage(chatMessage); err != nil {
		return fmt.Errorf("failed to create chat message: %w", err)
	}

	log.Printf("Persisted agent message for app %s: status=%s", appID, statusLevel)
	return nil
}

// executeRestartContainer restarts a Docker container
func (o *Orchestrator) executeRestartContainer(ctx context.Context, action RecommendedAction) error {
	// Extract container name
	containerName, ok := action.Parameters["container_name"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid container_name parameter")
	}

	appID, hasAppID := action.Parameters["app_id"].(string)

	// Restart the container with a 30-second timeout
	timeout := 30
	if err := o.dockerClient.ContainerRestart(ctx, containerName, container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		// Check if error is due to port conflict
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "port is already allocated") {
			log.Printf("ERROR: Cannot restart container %s due to port conflict: %v", containerName, err)

			// Persist an error message about the port conflict
			if hasAppID {
				errorMessage := database.ChatMessage{
					AppID:       appID,
					Timestamp:   time.Now(),
					Message:     fmt.Sprintf("Cannot restart container '%s' - port conflict detected. Another container is using the same port.", containerName),
					SenderType:  database.SenderTypeSystem,
					SenderName:  "Docker Manager",
					StatusLevel: sql.NullString{String: database.StatusLevelError, Valid: true},
					Details:     sql.NullString{String: errorMsg, Valid: true},
				}

				if err := database.CreateChatMessage(errorMessage); err != nil {
					log.Printf("WARNING: Failed to persist error notification: %v", err)
				}
			}

			// Return the error but mark it as handled
			return fmt.Errorf("port conflict for container %s: %w", containerName, err)
		}

		// For other errors, persist a generic error message
		if hasAppID {
			errorMessage := database.ChatMessage{
				AppID:       appID,
				Timestamp:   time.Now(),
				Message:     fmt.Sprintf("Failed to restart container '%s'", containerName),
				SenderType:  database.SenderTypeSystem,
				SenderName:  "Docker Manager",
				StatusLevel: sql.NullString{String: database.StatusLevelError, Valid: true},
				Details:     sql.NullString{String: errorMsg, Valid: true},
			}

			if err := database.CreateChatMessage(errorMessage); err != nil {
				log.Printf("WARNING: Failed to persist error notification: %v", err)
			}
		}

		return fmt.Errorf("failed to restart container %s: %w", containerName, err)
	}

	log.Printf("Successfully restarted container: %s", containerName)

	// Persist a success message about the restart
	if hasAppID {
		restartMessage := database.ChatMessage{
			AppID:       appID,
			Timestamp:   time.Now(),
			Message:     fmt.Sprintf("Container '%s' was successfully restarted", containerName),
			SenderType:  database.SenderTypeSystem,
			SenderName:  "Docker Manager",
			StatusLevel: sql.NullString{String: database.StatusLevelInfo, Valid: true},
		}

		if err := database.CreateChatMessage(restartMessage); err != nil {
			log.Printf("WARNING: Failed to persist restart notification: %v", err)
		}
	}

	return nil
}

// persistSetupMessage persists a setup progress message to the chat interface
func (o *Orchestrator) persistSetupMessage(appName, message, level string) {
	// Map our levels to new status levels
	var statusLevel string
	switch level {
	case "INFO":
		statusLevel = database.StatusLevelInfo
	case "ERROR":
		statusLevel = database.StatusLevelError
	default:
		statusLevel = database.StatusLevelInfo
	}

	// Create and persist the message
	chatMessage := database.ChatMessage{
		AppID:       strings.ToLower(appName),
		Timestamp:   time.Now(),
		Message:     message,
		SenderType:  database.SenderTypeSystem,
		SenderName:  "Setup Manager",
		StatusLevel: sql.NullString{String: statusLevel, Valid: true},
	}

	if err := database.CreateChatMessage(chatMessage); err != nil {
		log.Printf("Failed to persist setup message: %v", err)
	}
}

// createFallbackResponse creates a fallback response when LLM is unavailable
func (o *Orchestrator) createFallbackResponse(snapshot *SystemSnapshot, configs []AppConfig) *LLMResponse {
	response := &LLMResponse{
		OverallStatus:      StatusAllOK,
		Summary:            "System check completed (LLM unavailable - basic check only)",
		Analysis:           []AnalysisItem{},
		RecommendedActions: []RecommendedAction{},
	}

	// Check each app status
	for _, appStatus := range snapshot.AppStatuses {
		// Find the corresponding config
		var appConfig *AppConfig
		for _, cfg := range configs {
			if cfg.Name == appStatus.AppName {
				appConfig = &cfg
				break
			}
		}

		if appConfig == nil {
			continue
		}

		// Basic health check
		hasIssue := false
		issueMessage := ""

		// Check if services are running
		for _, service := range appStatus.ActualState.Services {
			if service.Status != ServiceStatusRunning {
				hasIssue = true
				issueMessage = fmt.Sprintf("Service %s is %s", service.Name, service.Status)
				response.OverallStatus = StatusCritical
				break
			}
			if service.RestartCount > 5 {
				hasIssue = true
				issueMessage = fmt.Sprintf("Service %s has high restart count: %d", service.Name, service.RestartCount)
				if response.OverallStatus != StatusCritical {
					response.OverallStatus = StatusWarning
				}
			}
		}

		// Create a chat message for each app
		statusLevel := database.StatusLevelInfo
		message := "All services running normally"

		if hasIssue {
			if response.OverallStatus == StatusCritical {
				statusLevel = database.StatusLevelCritical
			} else {
				statusLevel = database.StatusLevelWarning
			}
			message = issueMessage
		}

		response.RecommendedActions = append(response.RecommendedActions, RecommendedAction{
			ActionKey: ActionPersistChatMessage,
			Parameters: map[string]interface{}{
				"app_id":  appConfig.ID,
				"status":  statusLevel,
				"message": message,
			},
			Justification: "Regular system check",
		})
	}

	return response
}

// StartPeriodicChecks starts the periodic check loop for individual apps
func (o *Orchestrator) StartPeriodicChecks(ctx context.Context) {
	// Get all app configurations
	configs, err := o.configProvider.GetAll()
	if err != nil {
		log.Printf("ERROR: Failed to get app configurations: %v", err)
		return
	}

	// Create a ticker for each app with staggered start times
	for i, config := range configs {
		appID := config.ID
		// Stagger the initial checks to avoid all apps being checked at once
		initialDelay := time.Duration(i*10) * time.Second

		// Create a goroutine for each app's periodic checks
		go func(appID string, delay time.Duration) {
			// Wait for the initial delay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}

			// Run initial check for this app
			if err := o.RunCheckForApp(ctx, appID); err != nil {
				log.Printf("ERROR: Initial check failed for app %s: %v", appID, err)
			}

			// Create ticker for this app
			ticker := time.NewTicker(o.checkInterval)
			defer ticker.Stop()

			// Run periodic checks for this app
			for {
				select {
				case <-ctx.Done():
					log.Printf("Stopping periodic checks for app %s", appID)
					return
				case <-ticker.C:
					if err := o.RunCheckForApp(ctx, appID); err != nil {
						log.Printf("ERROR: Periodic check failed for app %s: %v", appID, err)
					}
				}
			}
		}(appID, initialDelay)
	}

	// Keep the main goroutine alive
	<-ctx.Done()
	log.Println("All periodic checks stopped")
}
