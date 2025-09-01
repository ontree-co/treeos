// Package system provides system monitoring and vitals functionality
package system

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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
	GPULoad        float64 // GPU utilization percentage (0-100)
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

	// Get GPU load
	gpuLoad := getGPULoad()

	return &Vitals{
		CPUPercent:     cpuUsage,
		MemPercent:     memStat.UsedPercent,
		DiskPercent:    diskStat.UsedPercent,
		NetworkRxBytes: rxBytes,
		NetworkTxBytes: txBytes,
		GPULoad:        gpuLoad,
	}, nil
}

// getGPULoad attempts to read GPU utilization from nvidia-smi or AMD tools
func getGPULoad() float64 {
	// Try NVIDIA GPU first
	if load := getNvidiaGPULoad(); load >= 0 {
		return load
	}

	// Try AMD GPU
	if load := getAMDGPULoad(); load >= 0 {
		return load
	}

	// No GPU detected or error reading GPU stats
	return 0
}

// getNvidiaGPULoad reads GPU utilization using nvidia-smi
func getNvidiaGPULoad() float64 {
	cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		// nvidia-smi not available or error
		return -1
	}

	// Parse the output (could be multiple GPUs, take first one)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		if value, err := strconv.ParseFloat(strings.TrimSpace(lines[0]), 64); err == nil {
			return value
		}
	}

	return -1
}

// getAMDGPULoad attempts to read AMD GPU utilization
func getAMDGPULoad() float64 {
	// Try reading from sysfs for AMD GPUs
	// This is a common location but may vary by system
	cmd := exec.Command("sh", "-c", "cat /sys/class/drm/card*/device/gpu_busy_percent 2>/dev/null | head -1")
	output, err := cmd.Output()
	if err != nil {
		return -1
	}

	if value, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); err == nil {
		return value
	}

	return -1
}
