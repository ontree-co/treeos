# Database Package

This package provides database connectivity and management for the OnTree application using SQLite.

## System Vitals Functions (Added 2025-07-10 - Usage Graph Ticket 5)

Added comprehensive database functions for managing system vital logs in `system_vitals.go`:

### Functions

- **GetMetricsLast24Hours(metricType string)**: Retrieves all system vitals from the last 24 hours
  - Returns data ordered by timestamp (ascending)
  - Returns empty slice if no data found (not an error)
  - The metricType parameter is included for future extensibility but currently returns all metrics

- **GetLatestMetric(metricType string)**: Gets the most recent system vital entry
  - Returns nil if no data exists (not an error condition)
  - Useful for displaying current values in the monitoring dashboard

- **StoreSystemVital(cpuPercent, memoryPercent, diskUsagePercent float64)**: Saves a new vital entry
  - Used by the system vitals collector to persist metrics

- **CleanupOldSystemVitals(olderThan time.Duration)**: Removes old entries
  - Implements data retention policy (e.g., keep only last 7 days)
  - Logs the number of entries cleaned up for debugging

- **GetMetricsForTimeRange(start, end time.Time)**: Flexible time range queries
  - Useful for future features like custom time range selection
  - Returns data ordered by timestamp (ascending)

### Performance Considerations

- The `system_vital_logs` table has an index on `timestamp` for efficient queries
- Queries use the index for both filtering and ordering operations
- All functions handle edge cases gracefully (no data, database not initialized)

### Testing

Comprehensive unit tests in `system_vitals_test.go` cover:
- Basic CRUD operations
- Edge cases (no data scenarios)
- Data ordering verification
- Time range queries
- Cleanup functionality

Run tests with: `go test ./internal/database -run TestSystemVitalsFunctions`