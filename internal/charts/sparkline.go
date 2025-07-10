package charts

import (
	"fmt"
	"html/template"
	"math"
	"strings"
)

// GenerateSparklineSVG creates an SVG string for a sparkline chart.
// dataPoints should be a slice of float64 values (e.g., percentages).
// width and height are the pixel dimensions of the SVG.
func GenerateSparklineSVG(dataPoints []float64, width, height int) template.HTML {
	if len(dataPoints) < 2 {
		return template.HTML("")
	}

	// Find min/max to scale the graph vertically
	minVal, maxVal := findMinMax(dataPoints)
	
	// If all values are the same, add a small range to avoid division by zero
	if minVal == maxVal {
		maxVal = minVal + 1
	}

	// Build the polyline points
	points := buildPolylinePoints(dataPoints, width, height, minVal, maxVal)

	// Generate SVG
	svg := fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none">
			<polyline fill="none" stroke="#007bff" stroke-width="2" points="%s" />
		</svg>`,
		width, height, width, height, points,
	)

	return template.HTML(svg)
}

// GenerateSparklineSVGWithStyle creates an SVG sparkline with custom styling options
func GenerateSparklineSVGWithStyle(dataPoints []float64, width, height int, strokeColor string, strokeWidth float64) template.HTML {
	if len(dataPoints) < 2 {
		return template.HTML("")
	}

	// Find min/max to scale the graph vertically
	minVal, maxVal := findMinMax(dataPoints)
	
	// If all values are the same, add a small range to avoid division by zero
	if minVal == maxVal {
		maxVal = minVal + 1
	}

	// Build the polyline points
	points := buildPolylinePoints(dataPoints, width, height, minVal, maxVal)

	// Generate SVG with custom styling
	svg := fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none">
			<polyline fill="none" stroke="%s" stroke-width="%.1f" points="%s" />
		</svg>`,
		width, height, width, height, strokeColor, strokeWidth, points,
	)

	return template.HTML(svg)
}

// findMinMax finds the minimum and maximum values in the data points
func findMinMax(dataPoints []float64) (float64, float64) {
	minVal := math.Inf(1)
	maxVal := math.Inf(-1)
	
	for _, v := range dataPoints {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	
	return minVal, maxVal
}

// buildPolylinePoints builds the SVG polyline points string
func buildPolylinePoints(dataPoints []float64, width, height int, minVal, maxVal float64) string {
	var points strings.Builder
	
	for i, dp := range dataPoints {
		// Calculate X position (evenly spaced)
		x := (float64(i) / float64(len(dataPoints)-1)) * float64(width)
		
		// Calculate Y position (scaled and inverted)
		// Normalize the value to 0-1 range
		normalized := (dp - minVal) / (maxVal - minVal)
		// Invert Y-axis (SVG coordinates start from top)
		y := float64(height) - (normalized * float64(height))
		
		// Add padding to prevent line from touching edges
		padding := float64(height) * 0.1
		y = padding + (y * 0.8)
		
		if i > 0 {
			points.WriteString(" ")
		}
		points.WriteString(fmt.Sprintf("%.2f,%.2f", x, y))
	}
	
	return points.String()
}

// GeneratePercentageSparkline creates a sparkline optimized for percentage values (0-100)
func GeneratePercentageSparkline(dataPoints []float64, width, height int) template.HTML {
	if len(dataPoints) < 2 {
		return template.HTML("")
	}

	// For percentages, we use a fixed 0-100 scale
	minVal := 0.0
	maxVal := 100.0

	// Build the polyline points
	points := buildPolylinePoints(dataPoints, width, height, minVal, maxVal)

	// Generate SVG
	svg := fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none">
			<polyline fill="none" stroke="#007bff" stroke-width="2" points="%s" />
		</svg>`,
		width, height, width, height, points,
	)

	return template.HTML(svg)
}