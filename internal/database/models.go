package database

import (
	"database/sql"
	"encoding/json"
	"time"
)

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

type SystemSetup struct {
	ID              int
	IsSetupComplete bool
	SetupDate       sql.NullTime
	NodeName        string
	NodeDescription sql.NullString
}

type SystemVitalLog struct {
	ID               int
	Timestamp        time.Time
	CPUPercent       float64
	MemoryPercent    float64
	DiskUsagePercent float64
}

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

const (
	// Operation Types
	OpTypePullImage         = "pull_image"
	OpTypeStartContainer    = "start_container"
	OpTypeCreateApp         = "create_app"
	OpTypeRecreateContainer = "recreate_container"
	OpTypeUpdateImage       = "update_image"

	// Status Choices
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)
