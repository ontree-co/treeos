// Package system provides system monitoring and vitals functionality
package system

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Vitals represents system resource usage information
type Vitals struct {
	CPUPercent     float64
	MemPercent     float64
	DiskPercent    float64
	NetworkRxBytes uint64 // Total bytes received across all interfaces
	NetworkTxBytes uint64 // Total bytes transmitted across all interfaces
}

// GetVitals retrieves current system resource usage information
func GetVitals() (*Vitals, error) {
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	cpuUsage := 0.0
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	memStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory usage: %w", err)
	}

	diskStat, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %w", err)
	}

	// Get network statistics - sum across all interfaces
	netStats, err := net.IOCounters(false) // false = sum all interfaces
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}

	var rxBytes, txBytes uint64
	if len(netStats) > 0 {
		// When pernic=false, gopsutil returns a single entry with summed stats
		rxBytes = netStats[0].BytesRecv
		txBytes = netStats[0].BytesSent
	}

	return &Vitals{
		CPUPercent:     cpuUsage,
		MemPercent:     memStat.UsedPercent,
		DiskPercent:    diskStat.UsedPercent,
		NetworkRxBytes: rxBytes,
		NetworkTxBytes: txBytes,
	}, nil
}
