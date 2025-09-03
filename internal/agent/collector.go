package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"ontree-node/internal/system"
)

// Collector is responsible for gathering system and application data
type Collector struct {
	dockerClient      *client.Client
	httpClient        *http.Client
	uptimeKumaBaseURL string
}

// NewCollector creates a new Collector instance
func NewCollector(uptimeKumaBaseURL string) (*Collector, error) {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test Docker connection
	ctx := context.Background()
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	return &Collector{
		dockerClient: dockerClient,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		uptimeKumaBaseURL: uptimeKumaBaseURL,
	}, nil
}

// Close cleans up resources
func (c *Collector) Close() error {
	if c.dockerClient != nil {
		return c.dockerClient.Close()
	}
	return nil
}

// CollectSystemSnapshot gathers all system and application data
func (c *Collector) CollectSystemSnapshot(configs []AppConfig) (*SystemSnapshot, error) {
	snapshot := &SystemSnapshot{
		Timestamp: time.Now(),
	}

	// Collect server health metrics
	serverHealth, err := c.collectServerHealth()
	if err != nil {
		return nil, fmt.Errorf("failed to collect server health: %w", err)
	}
	snapshot.ServerHealth = *serverHealth

	// Collect status for each application
	for _, config := range configs {
		appStatus, err := c.collectAppStatus(config)
		if err != nil {
			// Log error but continue with other apps
			fmt.Printf("Warning: Failed to collect status for app %s: %v\n", config.ID, err)
			continue
		}
		snapshot.AppStatuses = append(snapshot.AppStatuses, *appStatus)
	}

	return snapshot, nil
}

// CollectSystemSnapshotForApp gathers a snapshot for a single application
func (c *Collector) CollectSystemSnapshotForApp(config AppConfig) (*SystemSnapshot, error) {
	snapshot := &SystemSnapshot{
		Timestamp: time.Now(),
	}

	// Collect server health metrics (still needed for context)
	serverHealth, err := c.collectServerHealth()
	if err != nil {
		return nil, fmt.Errorf("failed to collect server health: %w", err)
	}
	snapshot.ServerHealth = *serverHealth

	// Collect status for just this application
	appStatus, err := c.collectAppStatus(config)
	if err != nil {
		return nil, fmt.Errorf("failed to collect status for app %s: %w", config.ID, err)
	}
	snapshot.AppStatuses = []AppStatus{*appStatus}

	return snapshot, nil
}

// collectServerHealth gathers server-level health metrics
func (c *Collector) collectServerHealth() (*ServerHealth, error) {
	// Use gopsutil to get system vitals (matching existing system package)
	vitals, err := system.GetVitals()
	if err != nil {
		return nil, fmt.Errorf("failed to get system vitals: %w", err)
	}

	return &ServerHealth{
		CPUUsagePercent:    vitals.CPUPercent,
		MemoryUsagePercent: vitals.MemPercent,
		DiskUsagePercent:   vitals.DiskPercent,
	}, nil
}

// collectAppStatus gathers status for a single application
func (c *Collector) collectAppStatus(config AppConfig) (*AppStatus, error) {
	appStatus := &AppStatus{
		AppID:   config.ID,
		AppName: config.Name,
		DesiredState: DesiredState{
			ExpectedServices: config.ExpectedServices,
		},
		ActualState: ActualState{
			Services: []ServiceStatus{},
		},
	}

	// Collect Docker container status for each expected service
	ctx := context.Background()
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Map to track which services we've found
	serviceStatuses := make(map[string]*ServiceStatus)

	// Look for containers matching the expected service names
	for _, service := range config.ExpectedServices {
		// Build expected container name using our naming convention: ontree-<app>-<service>-1
		// The app ID needs to be lowercase
		appIdentifier := strings.ToLower(config.ID)
		expectedName := fmt.Sprintf("ontree-%s-%s-1", appIdentifier, service)

		serviceStatus := &ServiceStatus{
			Name:   service,
			Status: ServiceStatusExited, // Default to exited if not found
			LogSummary: LogSummary{
				ErrorsFound:      0,
				SampleErrorLines: []string{},
			},
			ContainerName: expectedName, // Store the actual container name for restart actions
		}

		for _, cont := range containers {
			for _, name := range cont.Names {
				// Container names start with / in Docker API
				cleanName := strings.TrimPrefix(name, "/")
				if cleanName == expectedName {
					// Map Docker state to our status
					switch strings.ToLower(cont.State) {
					case "running":
						serviceStatus.Status = ServiceStatusRunning
					case "restarting":
						serviceStatus.Status = ServiceStatusRestarting
					default:
						serviceStatus.Status = ServiceStatusExited
					}

					// Get restart count from container inspection
					containerInfo, err := c.dockerClient.ContainerInspect(ctx, cont.ID)
					if err == nil {
						serviceStatus.RestartCount = containerInfo.RestartCount
					}

					// Collect logs if container is running
					if serviceStatus.Status == ServiceStatusRunning {
						logSummary, err := c.collectContainerLogs(cont.ID)
						if err == nil {
							serviceStatus.LogSummary = *logSummary
						}
					}
					break
				}
			}
		}

		serviceStatuses[service] = serviceStatus
	}

	// Add all service statuses to the app status
	for _, service := range config.ExpectedServices {
		if status, ok := serviceStatuses[service]; ok {
			appStatus.ActualState.Services = append(appStatus.ActualState.Services, *status)
		}
	}

	// Collect Uptime Kuma status if configured
	if config.UptimeKumaMonitor != "" && c.uptimeKumaBaseURL != "" {
		uptimeStatus, err := c.collectUptimeKumaStatus(config.UptimeKumaMonitor)
		if err != nil {
			// Log warning but don't fail
			fmt.Printf("Warning: Failed to get Uptime Kuma status for %s: %v\n", config.UptimeKumaMonitor, err)
			appStatus.ActualState.UptimeKumaStatus = UptimeKumaStatusDown
		} else {
			appStatus.ActualState.UptimeKumaStatus = uptimeStatus
		}
	} else {
		// Default to UP if Uptime Kuma is not configured
		appStatus.ActualState.UptimeKumaStatus = UptimeKumaStatusUp
	}

	return appStatus, nil
}

// collectContainerLogs fetches and analyzes recent logs from a container
func (c *Collector) collectContainerLogs(containerID string) (*LogSummary, error) {
	ctx := context.Background()

	// Get logs from the last 5 minutes
	since := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      since,
		Timestamps: false,
	}

	reader, err := c.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	// Keywords to search for in logs
	errorKeywords := []string{
		"ERROR", "FATAL", "Exception", "failed", "Failed",
		"error", "panic", "PANIC", "CRITICAL", "critical",
	}

	logSummary := &LogSummary{
		ErrorsFound:      0,
		SampleErrorLines: []string{},
	}

	// Scan logs line by line
	scanner := bufio.NewScanner(reader)
	maxSampleLines := 5 // Limit sample error lines

	for scanner.Scan() {
		line := scanner.Text()

		// Docker log format includes a header byte we need to strip
		if len(line) > 8 {
			// Strip the 8-byte header that Docker adds
			line = line[8:]
		}

		// Check for error keywords
		for _, keyword := range errorKeywords {
			if strings.Contains(line, keyword) {
				logSummary.ErrorsFound++

				// Add to sample if we haven't reached the limit
				if len(logSummary.SampleErrorLines) < maxSampleLines {
					// Truncate long lines
					if len(line) > 200 {
						line = line[:200] + "..."
					}
					logSummary.SampleErrorLines = append(logSummary.SampleErrorLines, line)
				}
				break // Don't count the same line multiple times
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning logs: %w", err)
	}

	return logSummary, nil
}

// collectUptimeKumaStatus queries Uptime Kuma for monitor status
func (c *Collector) collectUptimeKumaStatus(monitorName string) (string, error) {
	// Construct API URL
	// Note: This assumes Uptime Kuma's status page API
	// The actual endpoint may vary based on Uptime Kuma configuration
	url := fmt.Sprintf("%s/api/status-page/heartbeat/%s", c.uptimeKumaBaseURL, monitorName)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return UptimeKumaStatusDown, fmt.Errorf("failed to query Uptime Kuma: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UptimeKumaStatusDown, fmt.Errorf("Uptime Kuma returned status %d", resp.StatusCode)
	}

	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UptimeKumaStatusDown, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return UptimeKumaStatusDown, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check status field
	// Note: The exact field name depends on Uptime Kuma's API
	if status, ok := result["status"].(float64); ok {
		if status == 1 {
			return UptimeKumaStatusUp, nil
		}
	}

	return UptimeKumaStatusDown, nil
}
