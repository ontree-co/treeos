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

## Detailed Chart Generation

The `detailed.go` file implements full-featured charts with axes, labels, and grid lines:

### Main Functions

1. **GenerateDetailedChart(data DetailedChartData, width, height int) template.HTML**
   - Creates comprehensive charts with axes, labels, and grid lines
   - Supports time-series data with proper date/time formatting
   - Auto-scales Y-axis with smart padding
   - Respects min/max bounds for percentage metrics

### Features

- **Grid Lines**: 5 horizontal and 6 vertical lines for easy reading
- **Y-Axis**: Smart labels with unit formatting (%, MB/s, etc.)
- **X-Axis**: Time-based labels with intelligent date/time display
- **Data Visualization**: 
  - Line chart with filled area underneath
  - Data points shown as circles (for smaller datasets)
  - Responsive to data density
- **Clipping**: Chart data is clipped to prevent overflow

### Usage in Monitoring

Used by the monitoring handlers to display detailed metric views when users click on sparklines:
- CPU, Memory, Disk usage charts (0-100% scale)
- Network usage charts (variable scale)
- Supports time range selection (1h, 6h, 24h, 7d)