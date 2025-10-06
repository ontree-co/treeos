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
			<polyline fill="none" stroke="#198754" stroke-width="2" points="%s" />
		</svg>`,
		width, height, width, height, points,
	)

	//nolint:gosec // SVG generation, not user input
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

	//nolint:gosec // SVG generation, not user input
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
	// Pre-allocate capacity to avoid reallocation
	points.Grow(len(dataPoints) * 16) // Estimate ~16 chars per point

	// Pre-calculate constants
	xScale := float64(width) / float64(len(dataPoints)-1)
	yScale := float64(height) * 0.8
	yOffset := float64(height) * 0.9 // height - padding
	valueRange := maxVal - minVal

	for i, dp := range dataPoints {
		// Calculate X position (evenly spaced)
		x := float64(i) * xScale

		// Calculate Y position (scaled and inverted)
		// Normalize and invert in one step
		y := yOffset - ((dp - minVal) / valueRange * yScale)

		if i > 0 {
			points.WriteString(" ")
		}
		// Use more efficient formatting with 1 decimal place
		points.WriteString(fmt.Sprintf("%.1f,%.1f", x, y))
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

	// Generate SVG with optimized template
	var svg strings.Builder
	svg.Grow(256) // Pre-allocate for typical SVG size

	svg.WriteString(`<svg width="`)
	svg.WriteString(fmt.Sprintf("%d", width))
	svg.WriteString(`" height="`)
	svg.WriteString(fmt.Sprintf("%d", height))
	svg.WriteString(`" viewBox="0 0 `)
	svg.WriteString(fmt.Sprintf("%d %d", width, height))
	svg.WriteString(`" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none"><polyline fill="none" stroke="#198754" stroke-width="2" points="`)
	svg.WriteString(points)
	svg.WriteString(`"/></svg>`)

	//nolint:gosec // SVG generation, not user input
	return template.HTML(svg.String())
}
