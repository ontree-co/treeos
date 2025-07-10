package database

import (
	"database/sql"
	"fmt"
	"time"
)

// GetMetricsLast24Hours retrieves system vital logs for the specified metric type from the last 24 hours.
// metricType can be "cpu", "memory", or "disk"
func GetMetricsLast24Hours(metricType string) ([]SystemVitalLog, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Calculate the timestamp 24 hours ago
	since := time.Now().Add(-24 * time.Hour)

	// Query to get all system vitals from the last 24 hours
	query := `
		SELECT id, timestamp, cpu_percent, memory_percent, disk_usage_percent
		FROM system_vital_logs
		WHERE timestamp >= ?
		ORDER BY timestamp ASC
	`

	rows, err := db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []SystemVitalLog
	for rows.Next() {
		var m SystemVitalLog
		err := rows.Scan(&m.ID, &m.Timestamp, &m.CPUPercent, &m.MemoryPercent, &m.DiskUsagePercent)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// If no data found, return empty slice (not an error)
	if len(metrics) == 0 {
		return []SystemVitalLog{}, nil
	}

	return metrics, nil
}

// GetLatestMetric retrieves the most recent system vital log entry.
// Returns nil if no metrics are found (not an error condition).
func GetLatestMetric(metricType string) (*SystemVitalLog, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, timestamp, cpu_percent, memory_percent, disk_usage_percent
		FROM system_vital_logs
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var m SystemVitalLog
	err := db.QueryRow(query).Scan(&m.ID, &m.Timestamp, &m.CPUPercent, &m.MemoryPercent, &m.DiskUsagePercent)
	if err != nil {
		if err == sql.ErrNoRows {
			// No data is not an error, just return nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query latest metric: %w", err)
	}

	return &m, nil
}

// StoreSystemVital saves a new system vital log entry to the database.
func StoreSystemVital(cpuPercent, memoryPercent, diskUsagePercent float64) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		INSERT INTO system_vital_logs (cpu_percent, memory_percent, disk_usage_percent)
		VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, cpuPercent, memoryPercent, diskUsagePercent)
	if err != nil {
		return fmt.Errorf("failed to store system vital: %w", err)
	}

	return nil
}

// CleanupOldSystemVitals removes system vital logs older than the specified duration.
func CleanupOldSystemVitals(olderThan time.Duration) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	cutoff := time.Now().Add(-olderThan)

	query := `DELETE FROM system_vital_logs WHERE timestamp < ?`

	result, err := db.Exec(query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old vitals: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		// Log cleanup for debugging
		fmt.Printf("Cleaned up %d old system vital entries\n", rowsAffected)
	}

	return nil
}

// GetMetricsForTimeRange retrieves system vital logs within a specific time range.
// This is useful for custom time ranges beyond just 24 hours.
func GetMetricsForTimeRange(start, end time.Time) ([]SystemVitalLog, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, timestamp, cpu_percent, memory_percent, disk_usage_percent
		FROM system_vital_logs
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`

	rows, err := db.Query(query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics for time range: %w", err)
	}
	defer rows.Close()

	var metrics []SystemVitalLog
	for rows.Next() {
		var m SystemVitalLog
		err := rows.Scan(&m.ID, &m.Timestamp, &m.CPUPercent, &m.MemoryPercent, &m.DiskUsagePercent)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return metrics, nil
}

// MetricsBatch holds all metrics data for a time range
type MetricsBatch struct {
	Metrics []SystemVitalLog
}

// GetMetricsBatch retrieves all metrics for the given time range in a single query
// This is more efficient than making multiple queries for different metric types
func GetMetricsBatch(start, end time.Time) (*MetricsBatch, error) {
	metrics, err := GetMetricsForTimeRange(start, end)
	if err != nil {
		return nil, err
	}

	return &MetricsBatch{
		Metrics: metrics,
	}, nil
}

// ExtractMetricData extracts a specific metric type from the batch
func (b *MetricsBatch) ExtractMetricData(metricType string) []float64 {
	var data []float64

	for _, m := range b.Metrics {
		switch metricType {
		case "cpu":
			data = append(data, m.CPUPercent)
		case "memory":
			data = append(data, m.MemoryPercent)
		case "disk":
			data = append(data, m.DiskUsagePercent)
		}
	}

	return data
}
