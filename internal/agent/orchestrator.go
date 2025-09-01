package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	return &Orchestrator{
		configProvider:   configProvider,
		collector:        collector,
		reasoningService: reasoningService,
		dockerClient:     dockerClient,
		checkInterval:    config.CheckInterval,
	}, nil
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

// RunCheck performs a single check-analyze-act cycle
func (o *Orchestrator) RunCheck(ctx context.Context) error {
	log.Println("Starting agent check cycle...")

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

	// Step 2: Collect system snapshot
	snapshot, err := o.collector.CollectSystemSnapshot(configs)
	if err != nil {
		return fmt.Errorf("failed to collect system snapshot: %w", err)
	}

	log.Printf("Collected snapshot for %d applications", len(snapshot.AppStatuses))

	// Step 3: Analyze with LLM
	llmResponse, err := o.reasoningService.AnalyzeSnapshot(ctx, snapshot)
	if err != nil {
		// Log error but don't fail the entire check
		log.Printf("WARNING: Failed to analyze snapshot with LLM: %v", err)
		// Create a fallback response
		llmResponse = o.createFallbackResponse(snapshot, configs)
	}

	log.Printf("LLM analysis complete: status=%s, %d actions recommended",
		llmResponse.OverallStatus, len(llmResponse.RecommendedActions))

	// Step 4: Execute recommended actions
	if err := o.executeActions(ctx, llmResponse.RecommendedActions); err != nil {
		// Log error but continue - some actions may have succeeded
		log.Printf("WARNING: Some actions failed to execute: %v", err)
	}

	log.Println("Agent check cycle completed")
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

	// Create chat message
	chatMessage := database.ChatMessage{
		AppID:          appID,
		Timestamp:      time.Now(),
		StatusLevel:    status,
		MessageSummary: message,
	}

	// If there's additional detail, add it
	if action.Justification != "" {
		details := map[string]string{
			"justification": action.Justification,
		}
		detailsJSON, _ := json.Marshal(details)
		chatMessage.MessageDetails.String = string(detailsJSON)
		chatMessage.MessageDetails.Valid = true
	}

	// Save to database
	if err := database.CreateChatMessage(chatMessage); err != nil {
		return fmt.Errorf("failed to create chat message: %w", err)
	}

	log.Printf("Persisted chat message for app %s: status=%s", appID, status)
	return nil
}

// executeRestartContainer restarts a Docker container
func (o *Orchestrator) executeRestartContainer(ctx context.Context, action RecommendedAction) error {
	// Extract container name
	containerName, ok := action.Parameters["container_name"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid container_name parameter")
	}

	// Restart the container with a 30-second timeout
	timeout := 30
	if err := o.dockerClient.ContainerRestart(ctx, containerName, container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		return fmt.Errorf("failed to restart container %s: %w", containerName, err)
	}

	log.Printf("Successfully restarted container: %s", containerName)

	// Optionally persist a chat message about the restart
	if appID, ok := action.Parameters["app_id"].(string); ok {
		restartMessage := database.ChatMessage{
			AppID:          appID,
			Timestamp:      time.Now(),
			StatusLevel:    database.ChatStatusWarning,
			MessageSummary: fmt.Sprintf("Container '%s' was automatically restarted", containerName),
		}

		if err := database.CreateChatMessage(restartMessage); err != nil {
			log.Printf("WARNING: Failed to persist restart notification: %v", err)
		}
	}

	return nil
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
		status := database.ChatStatusOK
		message := "All services running normally"

		if hasIssue {
			if response.OverallStatus == StatusCritical {
				status = database.ChatStatusCritical
			} else {
				status = database.ChatStatusWarning
			}
			message = issueMessage
		}

		response.RecommendedActions = append(response.RecommendedActions, RecommendedAction{
			ActionKey: ActionPersistChatMessage,
			Parameters: map[string]interface{}{
				"app_id":  appConfig.ID,
				"status":  status,
				"message": message,
			},
			Justification: "Regular system check",
		})
	}

	return response
}

// StartPeriodicChecks starts the periodic check loop
func (o *Orchestrator) StartPeriodicChecks(ctx context.Context) {
	ticker := time.NewTicker(o.checkInterval)
	defer ticker.Stop()

	// Run initial check
	if err := o.RunCheck(ctx); err != nil {
		log.Printf("ERROR: Initial check failed: %v", err)
	}

	// Run periodic checks
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping periodic checks...")
			return
		case <-ticker.C:
			if err := o.RunCheck(ctx); err != nil {
				log.Printf("ERROR: Periodic check failed: %v", err)
			}
		}
	}
}
