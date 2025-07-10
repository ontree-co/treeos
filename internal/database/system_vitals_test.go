package database

import (
	"os"
	"testing"
	"time"
)

func TestSystemVitalsFunctions(t *testing.T) {
	// Create a temporary database for testing
	tempFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Initialize the database
	err = Initialize(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer Close()

	// Test StoreSystemVital
	t.Run("StoreSystemVital", func(t *testing.T) {
		err := StoreSystemVital(25.5, 60.3, 45.2)
		if err != nil {
			t.Errorf("Failed to store system vital: %v", err)
		}
	})

	// Test GetLatestMetric
	t.Run("GetLatestMetric", func(t *testing.T) {
		// Store another vital
		err := StoreSystemVital(30.2, 65.1, 48.9)
		if err != nil {
			t.Fatalf("Failed to store system vital: %v", err)
		}

		latest, err := GetLatestMetric("cpu")
		if err != nil {
			t.Errorf("Failed to get latest metric: %v", err)
		}

		if latest == nil {
			t.Error("Expected latest metric, got nil")
		} else {
			if latest.CPUPercent != 30.2 {
				t.Errorf("Expected CPU percent 30.2, got %f", latest.CPUPercent)
			}
			if latest.MemoryPercent != 65.1 {
				t.Errorf("Expected Memory percent 65.1, got %f", latest.MemoryPercent)
			}
			if latest.DiskUsagePercent != 48.9 {
				t.Errorf("Expected Disk percent 48.9, got %f", latest.DiskUsagePercent)
			}
		}
	})

	// Test GetMetricsLast24Hours
	t.Run("GetMetricsLast24Hours", func(t *testing.T) {
		// Add more test data
		for i := 0; i < 5; i++ {
			err := StoreSystemVital(float64(35+i), float64(70+i), float64(50+i))
			if err != nil {
				t.Fatalf("Failed to store system vital: %v", err)
			}
			time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
		}

		metrics, err := GetMetricsLast24Hours("cpu")
		if err != nil {
			t.Errorf("Failed to get metrics: %v", err)
		}

		// We should have at least 7 metrics (2 from previous tests + 5 new ones)
		if len(metrics) < 7 {
			t.Errorf("Expected at least 7 metrics, got %d", len(metrics))
		}

		// Verify they are ordered by timestamp
		for i := 1; i < len(metrics); i++ {
			if metrics[i].Timestamp.Before(metrics[i-1].Timestamp) {
				t.Error("Metrics are not ordered by timestamp")
			}
		}
	})

	// Test with no data (edge case)
	t.Run("GetLatestMetric_NoData", func(t *testing.T) {
		// Clear all data
		db := GetDB()
		_, err := db.Exec("DELETE FROM system_vital_logs")
		if err != nil {
			t.Fatalf("Failed to clear data: %v", err)
		}

		latest, err := GetLatestMetric("cpu")
		if err != nil {
			t.Errorf("Expected no error for no data, got: %v", err)
		}
		if latest != nil {
			t.Error("Expected nil for no data, got a metric")
		}
	})

	// Test GetMetricsLast24Hours with no data
	t.Run("GetMetricsLast24Hours_NoData", func(t *testing.T) {
		metrics, err := GetMetricsLast24Hours("cpu")
		if err != nil {
			t.Errorf("Expected no error for no data, got: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("Expected empty slice for no data, got %d metrics", len(metrics))
		}
	})

	// Test CleanupOldSystemVitals
	t.Run("CleanupOldSystemVitals", func(t *testing.T) {
		// Add some test data
		for i := 0; i < 10; i++ {
			err := StoreSystemVital(float64(40+i), float64(75+i), float64(55+i))
			if err != nil {
				t.Fatalf("Failed to store system vital: %v", err)
			}
		}

		// Clean up data older than 1 second (should remove nothing since we just added them)
		err := CleanupOldSystemVitals(1 * time.Second)
		if err != nil {
			t.Errorf("Failed to cleanup old vitals: %v", err)
		}

		// Verify data still exists
		metrics, _ := GetMetricsLast24Hours("cpu")
		if len(metrics) == 0 {
			t.Error("Data was incorrectly cleaned up")
		}
	})

	// Test GetMetricsForTimeRange
	t.Run("GetMetricsForTimeRange", func(t *testing.T) {
		// Clear data and add fresh test data
		db := GetDB()
		_, _ = db.Exec("DELETE FROM system_vital_logs")

		now := time.Now()
		// Add data with known timestamps
		for i := 0; i < 5; i++ {
			err := StoreSystemVital(float64(50+i), float64(80+i), float64(60+i))
			if err != nil {
				t.Fatalf("Failed to store system vital: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Query for a range that should include all data
		start := now.Add(-1 * time.Minute)
		end := now.Add(1 * time.Minute)
		metrics, err := GetMetricsForTimeRange(start, end)
		if err != nil {
			t.Errorf("Failed to get metrics for time range: %v", err)
		}

		if len(metrics) != 5 {
			t.Errorf("Expected 5 metrics, got %d", len(metrics))
		}
	})
}
