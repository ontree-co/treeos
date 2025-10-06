// Package runtime provides Docker-based application management.
package runtime

// ServiceStatus represents the running status of a service
type ServiceStatus string

const (
	// ServiceStatusRunning indicates the service is running
	ServiceStatusRunning ServiceStatus = "running"
	// ServiceStatusStopped indicates the service is stopped
	ServiceStatusStopped ServiceStatus = "stopped"
	// ServiceStatusError indicates the service encountered an error
	ServiceStatusError ServiceStatus = "error"
	// ServiceStatusUnknown indicates the service status is unknown
	ServiceStatusUnknown ServiceStatus = "unknown"
)

// AppStatus represents the aggregate status of an app
type AppStatus string

const (
	// AppStatusRunning indicates all services are running
	AppStatusRunning AppStatus = "running"
	// AppStatusPartial indicates some services are running
	AppStatusPartial AppStatus = "partial"
	// AppStatusStopped indicates all services are stopped
	AppStatusStopped AppStatus = "stopped"
	// AppStatusError indicates one or more services have errors
	AppStatusError AppStatus = "error"
	// AppStatusUnknown indicates status cannot be determined
	AppStatusUnknown AppStatus = "unknown"
)

// AppService represents a single service's runtime status
type AppService struct {
	Name   string        `json:"name"`
	Image  string        `json:"image"`
	Status ServiceStatus `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// CalculateAggregateStatus calculates the aggregate status based on service statuses
func CalculateAggregateStatus(services []AppService) AppStatus {
	if len(services) == 0 {
		return AppStatusUnknown
	}

	runningCount := 0
	errorCount := 0
	stoppedCount := 0

	for _, svc := range services {
		switch svc.Status {
		case ServiceStatusRunning:
			runningCount++
		case ServiceStatusError:
			errorCount++
		case ServiceStatusStopped:
			stoppedCount++
		}
	}

	// If any service has an error, the app is in error state
	if errorCount > 0 {
		return AppStatusError
	}

	// If all services are running, the app is running
	if runningCount == len(services) {
		return AppStatusRunning
	}

	// If all services are stopped, the app is stopped
	if stoppedCount == len(services) {
		return AppStatusStopped
	}

	// Otherwise, some services are running (partial state)
	return AppStatusPartial
}
