// Package runtime provides Docker-based application management.
package runtime

// ServiceStatus represents the running status of a service
type ServiceStatus string

const (
	ServiceStatusRunning ServiceStatus = "running"
	ServiceStatusStopped ServiceStatus = "stopped"
	ServiceStatusError   ServiceStatus = "error"
	ServiceStatusUnknown ServiceStatus = "unknown"
)

// AppStatus represents the aggregate status of an app
type AppStatus string

const (
	AppStatusRunning AppStatus = "running" // All services are running
	AppStatusPartial AppStatus = "partial" // Some services are running
	AppStatusStopped AppStatus = "stopped" // All services are stopped
	AppStatusError   AppStatus = "error"   // One or more services have errors
	AppStatusUnknown AppStatus = "unknown" // Status cannot be determined
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
