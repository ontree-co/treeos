package agent

import (
	"time"
)

// SystemSnapshot represents the full snapshot sent to the LLM
type SystemSnapshot struct {
	Timestamp    time.Time    `json:"timestamp"`
	ServerHealth ServerHealth `json:"server_health"`
	AppStatuses  []AppStatus  `json:"app_statuses"`
}

// ServerHealth represents the server's health metrics
type ServerHealth struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	DiskUsagePercent   float64 `json:"disk_usage_percent"`
}

// AppStatus represents the status of a single application
type AppStatus struct {
	AppName      string       `json:"app_name"`
	DesiredState DesiredState `json:"desired_state"`
	ActualState  ActualState  `json:"actual_state"`
}

// DesiredState represents what the application should be running
type DesiredState struct {
	ExpectedServices []string `json:"expected_services"`
}

// ActualState represents the current state of an application
type ActualState struct {
	UptimeKumaStatus string          `json:"uptime_kuma_status"` // "UP", "DOWN"
	Services         []ServiceStatus `json:"services"`
}

// ServiceStatus represents the status of a single service within an app
type ServiceStatus struct {
	Name         string     `json:"name"`
	Status       string     `json:"status"` // "running", "exited", "restarting"
	RestartCount int        `json:"restart_count"`
	LogSummary   LogSummary `json:"log_summary"`
}

// LogSummary represents a summary of log analysis
type LogSummary struct {
	ErrorsFound      int      `json:"errors_found"`
	SampleErrorLines []string `json:"sample_error_lines"`
}

// LLMResponse represents the structured response from the LLM
type LLMResponse struct {
	OverallStatus      string              `json:"overall_status"` // "ALL_OK", "WARNING", "CRITICAL"
	Summary            string              `json:"summary"`
	Analysis           []AnalysisItem      `json:"analysis"`
	RecommendedActions []RecommendedAction `json:"recommended_actions"`
}

// AnalysisItem represents a single analysis finding
type AnalysisItem struct {
	Component string `json:"component"`
	Status    string `json:"status"` // "OK", "WARN", "FAIL"
	Finding   string `json:"finding"`
}

// RecommendedAction represents an action recommended by the LLM
type RecommendedAction struct {
	ActionKey     string                 `json:"action_key"`
	Parameters    map[string]interface{} `json:"parameters"`
	Justification string                 `json:"justification"`
}

// Action keys for recommended actions
const (
	ActionPersistChatMessage = "PERSIST_CHAT_MESSAGE"
	ActionRestartContainer   = "RESTART_CONTAINER"
	ActionNoAction           = "NO_ACTION"
)

// Status levels for overall status and chat messages
const (
	StatusAllOK    = "ALL_OK"
	StatusWarning  = "WARNING"
	StatusCritical = "CRITICAL"
	StatusOK       = "OK"
	StatusWarn     = "WARN"
	StatusFail     = "FAIL"
)

// Service statuses
const (
	ServiceStatusRunning    = "running"
	ServiceStatusExited     = "exited"
	ServiceStatusRestarting = "restarting"
)

// Uptime Kuma statuses
const (
	UptimeKumaStatusUp   = "UP"
	UptimeKumaStatusDown = "DOWN"
)
