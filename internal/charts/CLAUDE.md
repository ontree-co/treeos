# Charts Package

This package provides SVG chart generation utilities for the monitoring dashboard.

## Sparkline Generation

The `sparkline.go` file implements SVG sparkline generation with the following features:

### Main Functions

1. **GenerateSparklineSVG(dataPoints []float64, width, height int) template.HTML**
   - Creates a basic sparkline with default blue color (#007bff)
   - Auto-scales to data range
   - Returns empty HTML if less than 2 data points

2. **GenerateSparklineSVGWithStyle(..., strokeColor string, strokeWidth float64) template.HTML**
   - Allows custom stroke color and width
   - Same auto-scaling behavior

3. **GeneratePercentageSparkline(dataPoints []float64, width, height int) template.HTML**
   - Optimized for percentage values (0-100 scale)
   - Fixed scale prevents jumpy visuals for percentage metrics

### Implementation Details

- SVG generation is done with string formatting (no external dependencies)
- Polyline points are calculated with proper Y-axis inversion (SVG coordinates)
- 10% padding is added to prevent lines from touching edges
- Handles edge cases: empty data, single point, all same values
- Returns `template.HTML` type for direct use in Go templates

### Usage Example

```go
// In a handler
dataPoints := []float64{15.2, 18.5, 14.3, 22.1, 19.8}
sparkline := charts.GenerateSparklineSVG(dataPoints, 150, 40)
// Use sparkline in template data
```

### Testing

Comprehensive unit tests cover:
- Empty and single data point handling
- Normal operation with various data ranges
- Custom styling
- SVG validity and structure
- Edge cases (negative values, same values)