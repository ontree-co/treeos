package charts

import (
	"html/template"
	"strings"
	"testing"
)

func TestGenerateSparklineSVG(t *testing.T) {
	tests := []struct {
		name       string
		dataPoints []float64
		width      int
		height     int
		wantEmpty  bool
		contains   []string
	}{
		{
			name:       "Empty data points",
			dataPoints: []float64{},
			width:      100,
			height:     40,
			wantEmpty:  true,
		},
		{
			name:       "Single data point",
			dataPoints: []float64{50},
			width:      100,
			height:     40,
			wantEmpty:  true,
		},
		{
			name:       "Two data points",
			dataPoints: []float64{20, 80},
			width:      100,
			height:     40,
			wantEmpty:  false,
			contains: []string{
				`<svg width="100" height="40"`,
				`viewBox="0 0 100 40"`,
				`stroke="#198754"`,
				`stroke-width="2"`,
				`<polyline`,
			},
		},
		{
			name:       "Multiple data points",
			dataPoints: []float64{10, 20, 15, 40, 35, 60, 55},
			width:      150,
			height:     50,
			wantEmpty:  false,
			contains: []string{
				`<svg width="150" height="50"`,
				`viewBox="0 0 150 50"`,
			},
		},
		{
			name:       "All same values",
			dataPoints: []float64{50, 50, 50, 50},
			width:      100,
			height:     40,
			wantEmpty:  false,
			contains: []string{
				`<svg`,
				`<polyline`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSparklineSVG(tt.dataPoints, tt.width, tt.height)

			if tt.wantEmpty && got != "" {
				t.Errorf("GenerateSparklineSVG() = %v, want empty", got)
			}

			if !tt.wantEmpty && got == "" {
				t.Errorf("GenerateSparklineSVG() returned empty, want SVG")
			}

			for _, substr := range tt.contains {
				if !strings.Contains(string(got), substr) {
					t.Errorf("GenerateSparklineSVG() missing substring %q in output", substr)
				}
			}
		})
	}
}

func TestGenerateSparklineSVGWithStyle(t *testing.T) {
	dataPoints := []float64{10, 30, 25, 45, 40}
	width := 100
	height := 40
	strokeColor := "#ff0000"
	strokeWidth := 3.5

	got := GenerateSparklineSVGWithStyle(dataPoints, width, height, strokeColor, strokeWidth)

	expectedSubstrings := []string{
		`stroke="#ff0000"`,
		`stroke-width="3.5"`,
		`<svg width="100" height="40"`,
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(string(got), substr) {
			t.Errorf("GenerateSparklineSVGWithStyle() missing substring %q in output", substr)
		}
	}
}

func TestFindMinMax(t *testing.T) {
	tests := []struct {
		name       string
		dataPoints []float64
		wantMin    float64
		wantMax    float64
	}{
		{
			name:       "Normal values",
			dataPoints: []float64{10, 20, 5, 30, 15},
			wantMin:    5,
			wantMax:    30,
		},
		{
			name:       "All same values",
			dataPoints: []float64{50, 50, 50},
			wantMin:    50,
			wantMax:    50,
		},
		{
			name:       "Negative values",
			dataPoints: []float64{-10, -5, -20, -15},
			wantMin:    -20,
			wantMax:    -5,
		},
		{
			name:       "Mixed positive and negative",
			dataPoints: []float64{-10, 0, 10, -5, 15},
			wantMin:    -10,
			wantMax:    15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := findMinMax(tt.dataPoints)
			if gotMin != tt.wantMin {
				t.Errorf("findMinMax() gotMin = %v, want %v", gotMin, tt.wantMin)
			}
			if gotMax != tt.wantMax {
				t.Errorf("findMinMax() gotMax = %v, want %v", gotMax, tt.wantMax)
			}
		})
	}
}

func TestBuildPolylinePoints(t *testing.T) {
	tests := []struct {
		name       string
		dataPoints []float64
		width      int
		height     int
		minVal     float64
		maxVal     float64
		wantPoints int // number of coordinate pairs
	}{
		{
			name:       "Two points",
			dataPoints: []float64{0, 100},
			width:      100,
			height:     40,
			minVal:     0,
			maxVal:     100,
			wantPoints: 2,
		},
		{
			name:       "Three points",
			dataPoints: []float64{25, 50, 75},
			width:      100,
			height:     40,
			minVal:     0,
			maxVal:     100,
			wantPoints: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPolylinePoints(tt.dataPoints, tt.width, tt.height, tt.minVal, tt.maxVal)

			// Count coordinate pairs
			coords := strings.Count(got, ",")
			if coords != tt.wantPoints {
				t.Errorf("buildPolylinePoints() returned %d coordinate pairs, want %d", coords, tt.wantPoints)
			}

			// Verify format (should contain numbers and commas)
			if !strings.Contains(got, ",") {
				t.Errorf("buildPolylinePoints() = %v, missing comma separators", got)
			}
		})
	}
}

func TestGeneratePercentageSparkline(t *testing.T) {
	tests := []struct {
		name       string
		dataPoints []float64
		width      int
		height     int
		wantEmpty  bool
	}{
		{
			name:       "Valid percentage values",
			dataPoints: []float64{0, 25, 50, 75, 100},
			width:      100,
			height:     40,
			wantEmpty:  false,
		},
		{
			name:       "Values exceeding 100",
			dataPoints: []float64{50, 75, 110, 80}, // Should still work
			width:      100,
			height:     40,
			wantEmpty:  false,
		},
		{
			name:       "Low values only",
			dataPoints: []float64{5, 10, 15, 20},
			width:      100,
			height:     40,
			wantEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePercentageSparkline(tt.dataPoints, tt.width, tt.height)

			if tt.wantEmpty && got != template.HTML("") {
				t.Errorf("GeneratePercentageSparkline() = %v, want empty", got)
			}

			if !tt.wantEmpty && got == template.HTML("") {
				t.Errorf("GeneratePercentageSparkline() returned empty, want SVG")
			}

			// Check for SVG structure
			if !tt.wantEmpty && !strings.Contains(string(got), "<svg") {
				t.Errorf("GeneratePercentageSparkline() missing SVG tag")
			}
		})
	}
}

// TestSVGValidity verifies that generated SVG is well-formed
func TestSVGValidity(t *testing.T) {
	dataPoints := []float64{10, 30, 20, 40, 35, 50}
	svg := GenerateSparklineSVG(dataPoints, 100, 40)

	svgStr := string(svg)

	// Check for proper SVG structure
	if !strings.HasPrefix(svgStr, "<svg") {
		t.Error("SVG should start with <svg tag")
	}

	if !strings.HasSuffix(strings.TrimSpace(svgStr), "</svg>") {
		t.Error("SVG should end with </svg> tag")
	}

	// Check for required attributes
	requiredAttrs := []string{
		"width=",
		"height=",
		"viewBox=",
		"xmlns=",
	}

	for _, attr := range requiredAttrs {
		if !strings.Contains(svgStr, attr) {
			t.Errorf("SVG missing required attribute: %s", attr)
		}
	}

	// Check polyline exists and has points
	if !strings.Contains(svgStr, "<polyline") {
		t.Error("SVG missing polyline element")
	}

	if !strings.Contains(svgStr, "points=") {
		t.Error("SVG polyline missing points attribute")
	}
}
