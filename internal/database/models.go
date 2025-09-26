package database

import (
	"database/sql"
	"encoding/json"
	"time"
)

// User represents a user in the system.
type User struct {
	ID          int
	Username    string
	Password    string
	Email       sql.NullString
	FirstName   sql.NullString
	LastName    sql.NullString
	IsStaff     bool
	IsSuperuser bool
	IsActive    bool
	DateJoined  time.Time
	LastLogin   sql.NullTime
}

// SystemSetup tracks the system setup state.
type SystemSetup struct {
	ID                 int
	IsSetupComplete    bool
	SetupDate          sql.NullTime
	NodeName           string
	NodeDescription    sql.NullString
	PublicBaseDomain   sql.NullString
	TailscaleAuthKey   sql.NullString
	TailscaleTags      sql.NullString
	AgentEnabled       sql.NullInt64
	AgentCheckInterval sql.NullString
	AgentLLMAPIKey     sql.NullString
	AgentLLMAPIURL     sql.NullString
	AgentLLMModel      sql.NullString
	UptimeKumaBaseURL  sql.NullString
	UpdateChannel      sql.NullString // "stable" or "beta", defaults to "beta"
}

// SystemVitalLog stores system performance metrics.
type SystemVitalLog struct {
	ID               int
	Timestamp        time.Time
	CPUPercent       float64
	MemoryPercent    float64
	DiskUsagePercent float64
	UploadRate       uint64 // bytes per second
	DownloadRate     uint64 // bytes per second
	GPULoad          float64
}

// ContainerOperation tracks container operation state and progress.
type ContainerOperation struct {
	ID              string
	OperationType   string
	AppName         string
	Status          string
	Progress        int
	ProgressMessage sql.NullString
	ErrorMessage    sql.NullString
	Metadata        json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     sql.NullTime
}

// ContainerOperationLog stores log entries for container operations.
type ContainerOperationLog struct {
	ID          int
	OperationID string
	Timestamp   time.Time
	Level       string
	Message     string
	Details     sql.NullString
}

// UpdateHistory tracks system update attempts
type UpdateHistory struct {
	ID           int
	Version      string
	Channel      string // "stable" or "beta"
	Status       string // "success", "failed", "rolled_back"
	ErrorMessage sql.NullString
	StartedAt    time.Time
	CompletedAt  sql.NullTime
	CreatedAt    time.Time
}

const (
	// OpTypePullImage indicates a container image pull operation.
	OpTypePullImage = "pull_image"
	// OpTypeStartContainer indicates a container start operation.
	OpTypeStartContainer = "start_container"
	// OpTypeCreateApp indicates an app creation operation.
	OpTypeCreateApp = "create_app"
	// OpTypeRecreateContainer indicates a container recreation operation.
	OpTypeRecreateContainer = "recreate_container"
	// OpTypeUpdateImage indicates an image update operation.
	OpTypeUpdateImage = "update_image"

	// StatusPending indicates an operation is waiting to start.
	StatusPending = "pending"
	// StatusInProgress indicates an operation is currently running.
	StatusInProgress = "in_progress"
	// StatusCompleted indicates an operation finished successfully.
	StatusCompleted = "completed"
	// StatusFailed indicates an operation failed.
	StatusFailed = "failed"

	// LogLevelDebug indicates a debug log level.
	LogLevelDebug = "debug"
	// LogLevelInfo indicates an info log level.
	LogLevelInfo = "info"
	// LogLevelWarning indicates a warning log level.
	LogLevelWarning = "warning"
	// LogLevelError indicates an error log level.
	LogLevelError = "error"

	// Sender types
	SenderTypeUser   = "user"
	SenderTypeAgent  = "agent"
	SenderTypeSystem = "system"

	// Status levels for agent monitoring messages
	StatusLevelInfo     = "info"
	StatusLevelWarning  = "warning"
	StatusLevelError    = "error"
	StatusLevelCritical = "critical"

	// Agent providers
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderLocal     = "local"
	ProviderOllama    = "ollama"
)
