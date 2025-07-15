package charts

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"
)

// TimeSeriesPoint represents a data point with timestamp for time-aware charts
type TimeSeriesPoint struct {
	Time  time.Time
	Value float64
}

// TimeSeriesOptions configures how time series data is rendered
type TimeSeriesOptions struct {
	Width         int
	Height        int
	StrokeColor   string
	StrokeWidth   float64
	GapThreshold  time.Duration // If gap between points exceeds this, break the line
	ShowNoData    bool          // Show "No data" message when empty
	MinValue      float64       // For fixed scale (e.g., 0-100 for percentages)
	MaxValue      float64       // For fixed scale
	UseFixedScale bool          // Whether to use MinValue/MaxValue or auto-scale
}

// DefaultSparklineOptions returns default options for sparklines
func DefaultSparklineOptions() TimeSeriesOptions {
	return TimeSeriesOptions{
		Width:        150,
		Height:       40,
		StrokeColor:  "#007bff",
		StrokeWidth:  2,
		GapThreshold: 2 * time.Minute, // 2x the normal collection interval of 60s
		ShowNoData:   true,
	}
}

// DefaultPercentageOptions returns default options for percentage sparklines
func DefaultPercentageOptions() TimeSeriesOptions {
	opts := DefaultSparklineOptions()
	opts.UseFixedScale = true
	opts.MinValue = 0
	opts.MaxValue = 100
	return opts
}

// GenerateTimeAwareSparkline creates a sparkline that respects time gaps
func GenerateTimeAwareSparkline(dataPoints []TimeSeriesPoint, startTime, endTime time.Time, opts TimeSeriesOptions) template.HTML {
	if len(dataPoints) == 0 {
		if opts.ShowNoData {
			return generateNoDataSVG(opts.Width, opts.Height, "No data")
		}
		return template.HTML("")
	}

	// If only one data point, show it as a single dot
	if len(dataPoints) == 1 {
		return generateSinglePointSVG(dataPoints[0], startTime, endTime, opts)
	}

	// Calculate time range
	timeRange := endTime.Sub(startTime)
	if timeRange <= 0 {
		timeRange = 24 * time.Hour // Default to 24 hours if invalid range
	}

	// Determine min/max values
	minVal, maxVal := opts.MinValue, opts.MaxValue
	if !opts.UseFixedScale {
		minVal, maxVal = findTimeSeriesMinMax(dataPoints)
		// Add padding
		valueRange := maxVal - minVal
		if valueRange == 0 {
			valueRange = 1
		}
		minVal -= valueRange * 0.1
		maxVal += valueRange * 0.1
	}

	// Group points into continuous segments based on gap threshold
	segments := groupIntoSegments(dataPoints, opts.GapThreshold)

	// Build SVG
	var svg strings.Builder
	svg.Grow(512) // Pre-allocate

	svg.WriteString(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none">`,
		opts.Width, opts.Height, opts.Width, opts.Height))

	// Draw each segment
	for _, segment := range segments {
		if len(segment) < 2 {
			// Single point - draw as a circle
			point := segment[0]
			x := calculateXPosition(point.Time, startTime, timeRange, opts.Width)
			y := calculateYPosition(point.Value, minVal, maxVal, opts.Height)
			svg.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="2" fill="%s"/>`,
				x, y, opts.StrokeColor))
		} else {
			// Multiple points - draw as polyline
			polyline := buildTimeAwarePolyline(segment, startTime, timeRange, minVal, maxVal, opts)
			svg.WriteString(fmt.Sprintf(`<polyline fill="none" stroke="%s" stroke-width="%.1f" points="%s"/>`,
				opts.StrokeColor, opts.StrokeWidth, polyline))
		}
	}

	svg.WriteString(`</svg>`)
	return template.HTML(svg.String())
}

// groupIntoSegments groups points into continuous segments based on time gaps
func groupIntoSegments(points []TimeSeriesPoint, gapThreshold time.Duration) [][]TimeSeriesPoint {
	if len(points) == 0 {
		return nil
	}

	var segments [][]TimeSeriesPoint
	currentSegment := []TimeSeriesPoint{points[0]}

	for i := 1; i < len(points); i++ {
		gap := points[i].Time.Sub(points[i-1].Time)
		if gap > gapThreshold {
			// Start new segment
			segments = append(segments, currentSegment)
			currentSegment = []TimeSeriesPoint{points[i]}
		} else {
			// Continue current segment
			currentSegment = append(currentSegment, points[i])
		}
	}

	// Add final segment
	if len(currentSegment) > 0 {
		segments = append(segments, currentSegment)
	}

	return segments
}

// buildTimeAwarePolyline builds polyline points with proper time-based positioning
func buildTimeAwarePolyline(points []TimeSeriesPoint, startTime time.Time, timeRange time.Duration, minVal, maxVal float64, opts TimeSeriesOptions) string {
	var polyline strings.Builder
	polyline.Grow(len(points) * 16)

	for i, point := range points {
		x := calculateXPosition(point.Time, startTime, timeRange, opts.Width)
		y := calculateYPosition(point.Value, minVal, maxVal, opts.Height)

		if i > 0 {
			polyline.WriteString(" ")
		}
		polyline.WriteString(fmt.Sprintf("%.1f,%.1f", x, y))
	}

	return polyline.String()
}

// calculateXPosition calculates the X coordinate based on time
func calculateXPosition(pointTime, startTime time.Time, timeRange time.Duration, width int) float64 {
	elapsed := pointTime.Sub(startTime)
	if elapsed < 0 {
		return 0
	}
	ratio := float64(elapsed) / float64(timeRange)
	if ratio > 1 {
		ratio = 1
	}
	return ratio * float64(width)
}

// calculateYPosition calculates the Y coordinate based on value
func calculateYPosition(value, minVal, maxVal float64, height int) float64 {
	if maxVal == minVal {
		return float64(height) / 2
	}
	// Normalize to 0-1
	normalized := (value - minVal) / (maxVal - minVal)
	// Clamp
	if normalized < 0 {
		normalized = 0
	} else if normalized > 1 {
		normalized = 1
	}
	// Invert (SVG Y axis is top-down)
	return float64(height) * (1 - normalized)
}

// findTimeSeriesMinMax finds min and max values in time series data
func findTimeSeriesMinMax(points []TimeSeriesPoint) (float64, float64) {
	if len(points) == 0 {
		return 0, 100
	}

	minVal := math.Inf(1)
	maxVal := math.Inf(-1)

	for _, p := range points {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
		}
	}

	return minVal, maxVal
}

// generateNoDataSVG generates an SVG with a "No data" message
func generateNoDataSVG(width, height int, message string) template.HTML {
	// For small sparklines, just show a flat dashed line
	if height <= 40 {
		return template.HTML(fmt.Sprintf(
			`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">
				<line x1="0" y1="%d" x2="%d" y2="%d" stroke="#dee2e6" stroke-width="1" stroke-dasharray="4,4"/>
			</svg>`,
			width, height, width, height, height/2, width, height/2))
	}

	// For larger charts, show the message
	return template.HTML(fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">
			<text x="%d" y="%d" text-anchor="middle" dominant-baseline="middle" 
				  font-size="12" fill="#6c757d">%s</text>
		</svg>`,
		width, height, width, height, width/2, height/2, message))
}

// generateSinglePointSVG generates an SVG for a single data point
func generateSinglePointSVG(point TimeSeriesPoint, startTime, endTime time.Time, opts TimeSeriesOptions) template.HTML {
	timeRange := endTime.Sub(startTime)
	if timeRange <= 0 {
		timeRange = 24 * time.Hour
	}

	x := calculateXPosition(point.Time, startTime, timeRange, opts.Width)

	// Determine Y position
	minVal, maxVal := opts.MinValue, opts.MaxValue
	if !opts.UseFixedScale {
		// For single point, use reasonable defaults
		minVal = point.Value - 10
		maxVal = point.Value + 10
		if minVal < 0 {
			minVal = 0
		}
	}

	y := calculateYPosition(point.Value, minVal, maxVal, opts.Height)

	return template.HTML(fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">
			<circle cx="%.1f" cy="%.1f" r="3" fill="%s"/>
		</svg>`,
		opts.Width, opts.Height, opts.Width, opts.Height, x, y, opts.StrokeColor))
}
