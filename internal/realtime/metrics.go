package realtime

import (
	"sync"
	"time"
)

// TimePoint represents a single metric measurement at a point in time
type TimePoint struct {
	Time  time.Time
	Value float64
}

// NetworkPoint represents network metrics at a point in time
type NetworkPoint struct {
	Time   time.Time
	RxBytes uint64
	TxBytes uint64
}

// Metrics stores real-time metrics in memory for the last 60 seconds
type Metrics struct {
	cpu     []TimePoint
	network []NetworkPoint
	mu      sync.RWMutex
}

// NewMetrics creates a new real-time metrics store
func NewMetrics() *Metrics {
	m := &Metrics{
		cpu:     make([]TimePoint, 0, 65), // Slightly larger capacity to avoid frequent reallocations
		network: make([]NetworkPoint, 0, 65),
	}
	
	// Start cleanup goroutine to remove data older than 60 seconds
	go m.cleanupLoop()
	
	return m
}

// AddCPU adds a CPU measurement
func (m *Metrics) AddCPU(value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cpu = append(m.cpu, TimePoint{
		Time:  time.Now(),
		Value: value,
	})
}

// AddNetwork adds network measurements
func (m *Metrics) AddNetwork(rxBytes, txBytes uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.network = append(m.network, NetworkPoint{
		Time:    time.Now(),
		RxBytes: rxBytes,
		TxBytes: txBytes,
	})
}

// GetCPU returns CPU measurements from the last duration
func (m *Metrics) GetCPU(duration time.Duration) []TimePoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	cutoff := time.Now().Add(-duration)
	var result []TimePoint
	
	// Find first point after cutoff
	for _, point := range m.cpu {
		if point.Time.After(cutoff) {
			result = append(result, point)
		}
	}
	
	return result
}

// GetNetwork returns network measurements from the last duration
func (m *Metrics) GetNetwork(duration time.Duration) []NetworkPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	cutoff := time.Now().Add(-duration)
	var result []NetworkPoint
	
	// Find first point after cutoff
	for _, point := range m.network {
		if point.Time.After(cutoff) {
			result = append(result, point)
		}
	}
	
	return result
}

// GetLatestCPU returns the most recent CPU measurement
func (m *Metrics) GetLatestCPU() (TimePoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.cpu) == 0 {
		return TimePoint{}, false
	}
	
	return m.cpu[len(m.cpu)-1], true
}

// GetLatestNetwork returns the most recent network measurement
func (m *Metrics) GetLatestNetwork() (NetworkPoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.network) == 0 {
		return NetworkPoint{}, false
	}
	
	return m.network[len(m.network)-1], true
}

// GetAverageCPU returns the average CPU usage over the specified duration
func (m *Metrics) GetAverageCPU(duration time.Duration) float64 {
	points := m.GetCPU(duration)
	if len(points) == 0 {
		return 0
	}
	
	var sum float64
	for _, p := range points {
		sum += p.Value
	}
	
	return sum / float64(len(points))
}

// cleanupLoop removes data older than 60 seconds
func (m *Metrics) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second) // Clean up every 10 seconds
	defer ticker.Stop()
	
	for range ticker.C {
		m.cleanup()
	}
}

// cleanup removes data older than 60 seconds
func (m *Metrics) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	cutoff := time.Now().Add(-60 * time.Second)
	
	// Find first CPU point to keep
	cpuStart := 0
	for i, point := range m.cpu {
		if point.Time.After(cutoff) {
			cpuStart = i
			break
		}
	}
	
	// Find first network point to keep  
	netStart := 0
	for i, point := range m.network {
		if point.Time.After(cutoff) {
			netStart = i
			break
		}
	}
	
	// Remove old data
	if cpuStart > 0 {
		m.cpu = m.cpu[cpuStart:]
	}
	if netStart > 0 {
		m.network = m.network[netStart:]
	}
}