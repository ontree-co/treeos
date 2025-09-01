// Package system provides system monitoring and vitals functionality
package system

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Variables for tracking network rate calculation
var (
	lastNetworkCheck     time.Time
	lastRxBytes          uint64
	lastTxBytes          uint64
	networkMutex         sync.Mutex
	lastUploadRate       uint64
	lastDownloadRate     uint64
)

// Vitals represents system resource usage information
type Vitals struct {
	CPUPercent   float64
	MemPercent   float64
	DiskPercent  float64
	UploadRate   uint64  // Bytes per second uploaded
	DownloadRate uint64  // Bytes per second downloaded
	GPULoad      float64 // GPU utilization percentage (0-100)
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

	// Calculate network rates
	uploadRate, downloadRate := getNetworkRates()

	// Get GPU load
	gpuLoad := getGPULoad()

	return &Vitals{
		CPUPercent:   cpuUsage,
		MemPercent:   memStat.UsedPercent,
		DiskPercent:  diskStat.UsedPercent,
		UploadRate:   uploadRate,
		DownloadRate: downloadRate,
		GPULoad:      gpuLoad,
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

// getNetworkRates calculates the upload and download rates in bytes per second
func getNetworkRates() (uint64, uint64) {
	networkMutex.Lock()
	defer networkMutex.Unlock()

	// Get current network statistics
	netStats, err := net.IOCounters(false) // false = sum all interfaces
	if err != nil || len(netStats) == 0 {
		// If we can't get stats, return the last known rates
		return lastUploadRate, lastDownloadRate
	}

	currentRxBytes := netStats[0].BytesRecv
	currentTxBytes := netStats[0].BytesSent
	currentTime := time.Now()

	// If this is the first call or it's been too long, just store values
	if lastNetworkCheck.IsZero() || currentTime.Sub(lastNetworkCheck) > 5*time.Minute {
		lastRxBytes = currentRxBytes
		lastTxBytes = currentTxBytes
		lastNetworkCheck = currentTime
		lastUploadRate = 0
		lastDownloadRate = 0
		return 0, 0
	}

	// Calculate time elapsed in seconds
	timeDelta := currentTime.Sub(lastNetworkCheck).Seconds()
	if timeDelta <= 0 {
		// No time has passed, return last known rates
		return lastUploadRate, lastDownloadRate
	}

	// Calculate rates (bytes per second)
	var uploadRate, downloadRate uint64

	// Handle counter overflow (counters can reset on reboot)
	if currentRxBytes >= lastRxBytes {
		downloadRate = uint64(float64(currentRxBytes-lastRxBytes) / timeDelta)
	}
	if currentTxBytes >= lastTxBytes {
		uploadRate = uint64(float64(currentTxBytes-lastTxBytes) / timeDelta)
	}

	// Update last values for next calculation
	lastRxBytes = currentRxBytes
	lastTxBytes = currentTxBytes
	lastNetworkCheck = currentTime
	lastUploadRate = uploadRate
	lastDownloadRate = downloadRate

	return uploadRate, downloadRate
}

// GetNetworkCounters returns the current cumulative network byte counters
// This is used for realtime metrics that calculate rates on-the-fly
func GetNetworkCounters() (uint64, uint64) {
	networkMutex.Lock()
	defer networkMutex.Unlock()
	
	// If we haven't initialized yet, get current values
	if lastNetworkCheck.IsZero() {
		netStats, err := net.IOCounters(false)
		if err != nil || len(netStats) == 0 {
			return 0, 0
		}
		lastRxBytes = netStats[0].BytesRecv
		lastTxBytes = netStats[0].BytesSent
		lastNetworkCheck = time.Now()
	}
	
	return lastRxBytes, lastTxBytes
}
