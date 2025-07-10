Part 1: Display Options (The Frontend with HTMX)

The core idea is to have the Go backend generate small, self-contained HTML fragments (partials) that HTMX can easily swap into the page. For charts, we'll have Go generate SVG images directly, which are just text and embed perfectly in HTML.
Option A: The "Dashboard" View (Recommended)

This is the most balanced approach, providing a quick overview with historical context in a very clean layout.

Visual Layout:
A 2x2 grid of "cards". Each card represents one metric (CPU, Memory, Disk, Network).

    Card Title: "CPU Load"

    Current Value: A large, easy-to-read percentage (e.g., "15.2%").

    24-Hour Sparkline: A small, simple line chart showing the trend over the last 24 hours. This is the key to showing history without a complex UI.

Mockup:
Generated code

+--------------------------------+ +--------------------------------+
| CPU Load | | Memory Usage |
| | | |
| 15.2% | | 45.8% |
| | | |
| /\_/\ /\ | | \_**_/\_ |
| _/ \/\_\_/ \_****\_\_\_\_******| | \_**\_/ \/\_****\_\_\_\_******|
| <---- 24 hours ago now ----> | | <---- 24 hours ago now ----> |
+--------------------------------+ +--------------------------------+
+--------------------------------+ +--------------------------------+
| Disk Usage (/home) | | Network Load |
| | | |
| 78.1% | | ↓ 1.2 MB/s ↑ 85 KB/s |
| | | |
| ****\_\_\_\_****/`| |      _/\_   _/\_/\             |
| ____________/`| | \_**\_/ \_/ \_****\_******|
| <---- 24 hours ago now ----> | | <---- 24 hours ago now ----> |
+--------------------------------+ +--------------------------------+

IGNORE_WHEN_COPYING_START
Use code with caution.
IGNORE_WHEN_COPYING_END

HTMX Implementation:

You'll have a main page that renders the grid. Each card will have an HTMX attribute to poll for updates.
Generated html

<!-- monitoring.html -->
<div class="grid-container">
    <!-- Each card is a partial that can be updated independently -->
    <div id="cpu-card" hx-get="/monitoring/partials/cpu" hx-trigger="every 5s" hx-swap="outerHTML">
        <!-- Initial content rendered on first page load -->
        {{ template "cpu-card-partial" .CPUData }}
    </div>

    <div id="mem-card" hx-get="/monitoring/partials/mem" hx-trigger="every 5s" hx-swap="outerHTML">
        {{ template "mem-card-partial" .MemData }}
    </div>

    <!-- ... and so on for disk and network ... -->

</div>

<!-- _cpu_card.html (the partial template) -->

{{ define "cpu-card-partial" }}

<div id="cpu-card" hx-get="/monitoring/partials/cpu" hx-trigger="every 5s" hx-swap="outerHTML">
    <h3>CPU Load</h3>
    <p class="current-value">{{ .CurrentLoad }}%</p>
    <div class="sparkline">
        <!-- The Go handler will generate this SVG markup -->
        {{ .SparklineSVG }}
    </div>
</div>
{{ end }}

IGNORE_WHEN_COPYING_START
Use code with caution. Html
IGNORE_WHEN_COPYING_END

When /monitoring/partials/cpu is called, the Go backend generates and returns only the HTML for that one card, which HTMX then swaps into the DOM.
Option B: The "Detailed View" on Click

This extends Option A. When a user clicks on a sparkline, it's replaced with a larger, more detailed graph for that specific metric, also rendered as an SVG.

HTMX Implementation:
Add an hx-get to the sparkline div itself.
Generated html

<!-- Inside the CPU card partial from Option A -->
<div class="sparkline" hx-get="/monitoring/charts/cpu" hx-target="#modal-content" hx-swap="innerHTML">
    <!-- The small SVG sparkline -->
    {{ .SparklineSVG }}
</div>

<!-- You would have a modal container somewhere in your main layout -->
<div id="modal">
    <div id="modal-content"></div>
</div>

IGNORE_WHEN_COPYING_START
Use code with caution. Html
IGNORE_WHEN_COPYING_END

The endpoint /monitoring/charts/cpu would return a larger, more detailed SVG chart with axes and labels, which HTMX would place into a modal dialog.
Part 2: Implementation Specification (The Go Backend)

Here’s how to build the Go components to support the frontend.

1. Data Collection (The Prober)

You need a library to get system stats. The de-facto standard in the Go ecosystem is gopsutil.
Generated bash

go get github.com/shirou/gopsutil/v3/cpu
go get github.com/shirou/gopsutil/v3/mem
go get github.com/shirou/gopsutil/v3/disk
go get github.com/shirou/gopsutil/v3/net

IGNORE_WHEN_COPYING_START
Use code with caution. Bash
IGNORE_WHEN_COPYING_END

Prober Logic:
Generated go

package monitoring

import (
"time"
"github.com/shirou/gopsutil/v3/cpu"
"github.com/shirou/gopsutil/v3/mem"
"github.com/shirou/gopsutil/v3/disk"
"github.com/shirou/gopsutil/v3/net"
)

// SystemMetrics holds a snapshot of system stats.
type SystemMetrics struct {
Timestamp time.Time
CPULoad float64 // percent
MemUsed float64 // percent
DiskUsed float64 // percent for a specific path
NetBytesIn uint64 // total bytes
NetBytesOut uint64 // total bytes
}

// Probe collects the current system metrics.
func Probe() (SystemMetrics, error) {
// CPU: Get overall percentage over a short interval.
cpuPercentages, err := cpu.Percent(time.Second, false)
if err != nil {
return SystemMetrics{}, err
}
// Memory
vm, err := mem.VirtualMemory()
if err != nil {
return SystemMetrics{}, err
}
// Disk: Specify the path you want to monitor.
d, err := disk.Usage("/") // Or "/home", etc.
if err != nil {
return SystemMetrics{}, err
}
// Network: Get cumulative counters. We'll calculate the rate later.
netIO, err := net.IOCounters(false)
if err != nil {
return SystemMetrics{}, err
}
metrics := SystemMetrics{
Timestamp: time.Now(),
CPULoad: cpuPercentages[0],
MemUsed: vm.UsedPercent,
DiskUsed: d.UsedPercent,
NetBytesIn: netIO[0].BytesRecv,
NetBytesOut: netIO[0].BytesSent,
}
return metrics, nil
}

IGNORE_WHEN_COPYING_START
Use code with caution. Go
IGNORE_WHEN_COPYING_END 2. Data Storage

For a small, self-contained server, an embedded database is perfect.

    Recommendation: bbolt (a pure Go key-value store, fork of BoltDB). It's simple, requires no CGO, and is very fast for this use case. SQLite is also a great choice if you prefer SQL.

Generated bash

go get go.etcd.io/bbolt/...

IGNORE_WHEN_COPYING_START
Use code with caution. Bash
IGNORE_WHEN_COPYING_END

Storage Logic:
You'll store time-series data. A good bucket structure in bbolt would be one bucket per metric type.
Generated go

// db.go
package monitoring

import (
"encoding/binary"
"time"
"go.etcd.io/bbolt"
)

// StoreMetrics saves a SystemMetrics snapshot to the database.
func (s *YourServer) StoreMetrics(metrics SystemMetrics) error {
return s.db.Update(func(tx *bbolt.Tx) error {
// Keys will be timestamps, values will be the metric value.
// Use BigEndian so keys are sorted chronologically.
key := make([]byte, 8)
binary.BigEndian.PutUint64(key, uint64(metrics.Timestamp.UnixNano()))
// Store each metric in its own bucket
b, err := tx.CreateBucketIfNotExists([]byte("cpu"))
if err != nil { return err }
b.Put(key, floatToBytes(metrics.CPULoad)) // You'll need helper funcs for conversion

    	// ... repeat for mem, disk, net_in, net_out ...

    	return nil
    })

}

// GetMetricsLast24Hours retrieves data points for the last 24 hours.
func (s \*YourServer) GetMetricsLast24Hours(bucketName string) ([]DataPoint, error) {
// ... logic to open a transaction, get the bucket,
// and iterate over keys from (time.Now() - 24h) to time.Now() ...
}

IGNORE_WHEN_COPYING_START
Use code with caution. Go
IGNORE_WHEN_COPYING_END 3. The Recurring Job

Use a goroutine with a time.Ticker to run the prober periodically (e.g., every minute).

In your main.go or server startup:
Generated go

func (s _YourServer) startMonitoringJob() {
log.Println("Starting monitoring job...")
// Probe every 1 minute.
ticker := time.NewTicker(1 _ time.Minute)
go func() {
for range ticker.C {
metrics, err := monitoring.Probe()
if err != nil {
log.Printf("Error probing system: %v", err)
continue
}
if err := s.StoreMetrics(metrics); err != nil {
log.Printf("Error storing metrics: %v", err)
}
}
}()
}

IGNORE_WHEN_COPYING_START
Use code with caution. Go
IGNORE_WHEN_COPYING_END 4. HTTP Handlers & SVG Generation

This is where you tie everything together for HTMX.

SVG Sparkline Generation:
This is the "magic" part. You don't need a heavy library. A simple function can generate the SVG string.
Generated go

// svg.go
package monitoring

import (
"fmt"
"html/template"
"strings"
)

// GenerateSparklineSVG creates an SVG string for a sparkline.
// DataPoints should be a slice of float64 values (e.g., percentages).
// Width and Height are the pixel dimensions of the SVG.
func GenerateSparklineSVG(dataPoints []float64, width, height int) template.HTML {
if len(dataPoints) < 2 {
return "" // Not enough data to draw a line
}

    // Find min/max to scale the graph vertically
    maxVal := -1.0
    for _, v := range dataPoints {
    	if v > maxVal { maxVal = v }
    }
    // Let's assume a 0-100 scale for simplicity (e.g. percentages)
    maxVal = 100.0

    // Build the polyline points attribute (e.g., "0,50 10,45 20,48...")
    var points strings.Builder
    for i, dp := range dataPoints {
    	x := (float64(i) / float64(len(dataPoints)-1)) * float64(width)
    	y := float64(height) - ((dp / maxVal) * float64(height)) // Invert Y-axis
    	points.WriteString(fmt.Sprintf("%.2f,%.2f ", x, y))
    }

    svg := fmt.Sprintf(
    	`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none">
    		<polyline fill="none" stroke="#007bff" stroke-width="2" points="%s" />
    	</svg>`,
    	width, height, width, height, points.String(),
    )

    return template.HTML(svg)

}

IGNORE_WHEN_COPYING_START
Use code with caution. Go
IGNORE_WHEN_COPYING_END

Handler for a Partial:
Generated go

// handlers.go

// Network load needs special handling since we store cumulative bytes.
func calculateRate(current, previous uint64, interval time.Duration) float64 {
if interval.Seconds() == 0 {
return 0
}
// bytes per second
return float64(current-previous) / interval.Seconds()
}

func (s *YourServer) handleCPUPartial(w http.ResponseWriter, r *http.Request) {
// 1. Get latest CPU value from DB.
latestCPU := s.db.GetLatest("cpu")

    // 2. Get last 24 hours of CPU data from DB.
    historicalData, _ := s.db.GetMetricsLast24Hours("cpu")

    // 3. Generate the sparkline.
    sparklineSVG := monitoring.GenerateSparklineSVG(historicalData, 150, 40) // width, height

    // 4. Prepare data for the template.
    data := struct {
    	CurrentLoad  string
    	SparklineSVG template.HTML
    }{
    	CurrentLoad:  fmt.Sprintf("%.1f", latestCPU),
    	SparklineSVG: sparklineSVG,
    }

    // 5. Render the partial template and send it.
    s.templates.ExecuteTemplate(w, "cpu-card-partial", data)

}

IGNORE_WHEN_COPYING_START
Use code with caution. Go
IGNORE_WHEN_COPYING_END

This approach gives you a highly efficient, modern, and minimal server monitoring dashboard that perfectly fits your specified technology stack.
