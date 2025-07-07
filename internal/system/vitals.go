// Package system provides system monitoring and vitals functionality
package system

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Vitals represents system resource usage information
type Vitals struct {
	CPUPercent  float64
	MemPercent  float64
	DiskPercent float64
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

	return &Vitals{
		CPUPercent:  cpuUsage,
		MemPercent:  memStat.UsedPercent,
		DiskPercent: diskStat.UsedPercent,
	}, nil
}
