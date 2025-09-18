package charts

import (
	"fmt"
	"html/template"
	"math"
	"time"
)

// DetailedChartData holds all the data needed to render a detailed chart
type DetailedChartData struct {
	Points    []DataPoint
	Title     string
	YAxisUnit string // e.g., "%", "MB/s"
	MinValue  float64
	MaxValue  float64
	StartTime time.Time // Requested start time (for showing full range)
	EndTime   time.Time // Requested end time
}

// DataPoint represents a single data point with timestamp
type DataPoint struct {
	Time  time.Time
	Value float64
}

// GenerateDetailedChart creates a detailed SVG chart with axes, labels, and grid lines
func GenerateDetailedChart(data DetailedChartData, width, height int) template.HTML {
	if len(data.Points) == 0 {
		return template.HTML(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">
			<text x="%d" y="%d" text-anchor="middle" fill="#6c757d">No data available</text>
		</svg>`, width, height, width, height, width/2, height/2))
	}

	// Chart margins
	marginTop := 40
	marginRight := 60
	marginBottom := 60
	marginLeft := 70

	chartWidth := width - marginLeft - marginRight
	chartHeight := height - marginTop - marginBottom

	// Calculate min/max if not provided
	if data.MinValue == 0 && data.MaxValue == 0 {
		data.MinValue, data.MaxValue = findDataPointMinMax(data.Points)
	}

	// Add padding to min/max
	valueRange := data.MaxValue - data.MinValue
	if valueRange == 0 {
		valueRange = 1
	}
	padding := valueRange * 0.1
	data.MinValue -= padding
	data.MaxValue += padding

	// Ensure min is at least 0 for percentage metrics
	if data.YAxisUnit == "%" && data.MinValue < 0 {
		data.MinValue = 0
	}
	if data.YAxisUnit == "%" && data.MaxValue > 100 {
		data.MaxValue = 100
	}

	// Start building SVG
	svg := fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">`, width, height, width, height)

	// Background
	svg += fmt.Sprintf(`<rect width="%d" height="%d" fill="#ffffff"/>`, width, height)

	// Title
	svg += fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold" fill="#2d3748">%s</text>`, width/2, data.Title)

	// Grid lines
	svg += generateGridLines(marginLeft, marginTop, chartWidth, chartHeight)

	// Y-axis labels and ticks
	svg += generateYAxis(marginLeft, marginTop, chartHeight, data.MinValue, data.MaxValue, data.YAxisUnit)

	// X-axis labels and ticks
	svg += generateXAxis(marginLeft, marginTop, chartWidth, chartHeight, data.StartTime, data.EndTime)

	// Plot area clipping
	svg += fmt.Sprintf(`<defs><clipPath id="plotArea"><rect x="%d" y="%d" width="%d" height="%d"/></clipPath></defs>`,
		marginLeft, marginTop, chartWidth, chartHeight)

	// Data line
	svg += `<g clip-path="url(#plotArea)">`
	svg += generateDataLine(data.Points, marginLeft, marginTop, chartWidth, chartHeight, data.MinValue, data.MaxValue, data.StartTime, data.EndTime)
	svg += `</g>`

	// Chart border
	svg += fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="#dee2e6" stroke-width="1"/>`,
		marginLeft, marginTop, chartWidth, chartHeight)

	svg += `</svg>`

	return template.HTML(svg)
}

// generateGridLines creates horizontal and vertical grid lines
func generateGridLines(left, top, width, height int) string {
	svg := `<g stroke="#f0f0f0" stroke-width="1">`

	// Horizontal grid lines (5 lines)
	for i := 0; i <= 5; i++ {
		y := top + (height * i / 5)
		svg += fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d"/>`, left, y, left+width, y)
	}

	// Vertical grid lines (6 lines)
	for i := 0; i <= 6; i++ {
		x := left + (width * i / 6)
		svg += fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d"/>`, x, top, x, top+height)
	}

	svg += `</g>`
	return svg
}

// generateYAxis creates Y-axis labels and ticks
func generateYAxis(left, top, height int, minVal, maxVal float64, unit string) string {
	svg := `<g font-size="12" fill="#6c757d">`

	// Generate 6 labels (including top and bottom)
	for i := 0; i <= 5; i++ {
		y := top + height - (height * i / 5)
		value := minVal + (maxVal-minVal)*float64(i)/5

		label := fmt.Sprintf("%.1f%s", value, unit)
		if unit == "%" {
			label = fmt.Sprintf("%.0f%s", value, unit)
		}

		svg += fmt.Sprintf(`<text x="%d" y="%d" text-anchor="end" dominant-baseline="middle">%s</text>`,
			left-10, y, label)

		// Tick mark
		svg += fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#dee2e6"/>`,
			left-5, y, left, y)
	}

	svg += `</g>`
	return svg
}

// generateXAxis creates X-axis labels and ticks based on requested time range
func generateXAxis(left, top, width, height int, startTime, endTime time.Time) string {
	svg := `<g font-size="11" fill="#6c757d">`

	// Calculate time range
	timeRange := endTime.Sub(startTime)
	if timeRange <= 0 {
		timeRange = 24 * time.Hour // Default
	}

	// Generate evenly spaced time labels across the time range
	labelCount := 7
	if timeRange < 1*time.Hour {
		labelCount = 5 // Fewer labels for short time ranges
	}

	for i := 0; i < labelCount; i++ {
		// Calculate time for this label position
		labelTime := startTime.Add(timeRange * time.Duration(i) / time.Duration(labelCount-1))

		// Calculate X position
		elapsed := labelTime.Sub(startTime)
		x := left + int(float64(elapsed)/float64(timeRange)*float64(width))
		y := top + height + 20

		// Format time label based on time range
		var timeLabel string
		if timeRange <= 2*time.Hour {
			// Show time only for short ranges
			timeLabel = labelTime.Format("15:04")
		} else if timeRange <= 24*time.Hour {
			// Show time with abbreviated date
			if i == 0 || i == labelCount-1 {
				timeLabel = labelTime.Format("Jan 2 15:04")
			} else {
				timeLabel = labelTime.Format("15:04")
			}
		} else {
			// Show date and time for longer ranges
			timeLabel = labelTime.Format("Jan 2 15:04")
		}

		svg += fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle">%s</text>`, x, y, timeLabel)

		// Tick mark
		svg += fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#dee2e6"/>`,
			x, top+height, x, top+height+5)
	}

	svg += `</g>`
	return svg
}

// generateDataLine creates the actual data line with gap detection
func generateDataLine(points []DataPoint, left, top, width, height int, minVal, maxVal float64, requestedStartTime, requestedEndTime time.Time) string {
	if len(points) == 0 {
		return ""
	}

	// Single point - just draw a dot
	if len(points) == 1 {
		point := points[0]
		x := left + width/2 // Center it
		normalized := (point.Value - minVal) / (maxVal - minVal)
		y := top + height - int(normalized*float64(height))
		return fmt.Sprintf(`<circle cx="%d" cy="%d" r="4" fill="#198754"/>`, x, y)
	}

	// Use requested time range for positioning
	timeRange := requestedEndTime.Sub(requestedStartTime)
	if timeRange <= 0 {
		timeRange = 24 * time.Hour // Default
	}

	// Gap threshold - if gap is more than 2x the expected interval, break the line
	// For 60s collection interval, gap threshold is 2 minutes
	gapThreshold := 2 * time.Minute

	// Group points into segments based on time gaps
	segments := [][]DataPoint{}
	currentSegment := []DataPoint{points[0]}

	for i := 1; i < len(points); i++ {
		gap := points[i].Time.Sub(points[i-1].Time)
		if gap > gapThreshold {
			// Start new segment
			segments = append(segments, currentSegment)
			currentSegment = []DataPoint{points[i]}
		} else {
			// Continue current segment
			currentSegment = append(currentSegment, points[i])
		}
	}
	// Add final segment
	if len(currentSegment) > 0 {
		segments = append(segments, currentSegment)
	}

	svg := ""

	// Draw each segment separately
	for _, segment := range segments {
		if len(segment) < 2 {
			// Single point segment - draw as dot
			point := segment[0]
			elapsed := point.Time.Sub(requestedStartTime)
			x := left + int(float64(elapsed)/float64(timeRange)*float64(width))
			normalized := (point.Value - minVal) / (maxVal - minVal)
			y := top + height - int(normalized*float64(height))
			svg += fmt.Sprintf(`<circle cx="%d" cy="%d" r="3" fill="#198754"/>`, x, y)
			continue
		}

		// Build polyline for this segment
		polylinePoints := ""
		firstX := 0
		lastX := 0

		for i, point := range segment {
			// Calculate X based on actual timestamp
			elapsed := point.Time.Sub(requestedStartTime)
			x := left + int(float64(elapsed)/float64(timeRange)*float64(width))

			if i == 0 {
				firstX = x
			}
			lastX = x

			// Calculate Y based on value
			normalized := (point.Value - minVal) / (maxVal - minVal)
			if normalized < 0 {
				normalized = 0
			} else if normalized > 1 {
				normalized = 1
			}
			y := top + height - int(normalized*float64(height))

			if i > 0 {
				polylinePoints += " "
			}
			polylinePoints += fmt.Sprintf("%d,%d", x, y)
		}

		// Create filled area under this segment
		areaPoints := polylinePoints
		areaPoints += fmt.Sprintf(" %d,%d", lastX, top+height)
		areaPoints += fmt.Sprintf(" %d,%d", firstX, top+height)

		svg += fmt.Sprintf(`<polygon points="%s" fill="#198754" fill-opacity="0.1"/>`, areaPoints)
		svg += fmt.Sprintf(`<polyline points="%s" fill="none" stroke="#198754" stroke-width="2"/>`, polylinePoints)

		// Add dots for small segments
		if len(segment) < 20 {
			for _, point := range segment {
				elapsed := point.Time.Sub(requestedStartTime)
				x := left + int(float64(elapsed)/float64(timeRange)*float64(width))
				normalized := (point.Value - minVal) / (maxVal - minVal)
				if normalized < 0 {
					normalized = 0
				} else if normalized > 1 {
					normalized = 1
				}
				y := top + height - int(normalized*float64(height))
				svg += fmt.Sprintf(`<circle cx="%d" cy="%d" r="3" fill="#198754"/>`, x, y)
			}
		}
	}

	return svg
}

// findDataPointMinMax finds min and max values from DataPoint slice
func findDataPointMinMax(points []DataPoint) (float64, float64) {
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
