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
	ID              int
	IsSetupComplete bool
	SetupDate       sql.NullTime
	NodeName        string
	NodeDescription sql.NullString
}

// SystemVitalLog stores system performance metrics.
type SystemVitalLog struct {
	ID               int
	Timestamp        time.Time
	CPUPercent       float64
	MemoryPercent    float64
	DiskUsagePercent float64
}

// DockerOperation tracks Docker operation state and progress.
type DockerOperation struct {
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

// DockerOperationLog stores log entries for Docker operations.
type DockerOperationLog struct {
	ID          int
	OperationID string
	Timestamp   time.Time
	Level       string
	Message     string
	Details     sql.NullString
}

const (
	// OpTypePullImage indicates a Docker image pull operation.
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
)

// DeployedApp represents a deployed application managed by onTree.
type DeployedApp struct {
	ID            string
	Name          string
	DockerCompose string
	Subdomain     string
	HostPort      int  // The port exposed ON THE HOST (e.g., 8080)
	IsExposed     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
