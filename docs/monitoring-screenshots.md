# Monitoring Feature Screenshots Guide

This document describes the screenshots that should be taken to document the monitoring feature.

## Required Screenshots

### 1. Monitoring Dashboard Overview
**Filename**: `monitoring-dashboard.png`
**Description**: Full view of the monitoring dashboard showing the 2x2 grid layout
**Elements to capture**:
- All four metric cards (CPU, Memory, Disk, Network)
- Current values displayed prominently
- 24-hour sparklines visible
- Navigation menu showing "Monitoring" item

### 2. CPU Sparkline Detail
**Filename**: `monitoring-cpu-sparkline.png`
**Description**: Close-up of the CPU Load card
**Elements to capture**:
- Card title "CPU Load"
- Current percentage value (e.g., "15.2%")
- Blue sparkline showing 24-hour trend
- Clean card styling

### 3. Detailed Chart Modal
**Filename**: `monitoring-detailed-chart.png`
**Description**: Modal window showing detailed CPU chart
**Elements to capture**:
- Modal title showing metric name
- Time range buttons (1h, 6h, 24h, 7d) with 24h selected
- Large 700x400 SVG chart with:
  - Y-axis labels (0%, 25%, 50%, 75%, 100%)
  - X-axis time labels
  - Grid lines
  - Filled area under the line
  - Data points (if visible)

### 4. Mobile Responsive View
**Filename**: `monitoring-mobile.png`
**Description**: Dashboard on mobile device
**Elements to capture**:
- Single column layout
- Cards stacked vertically
- Properly sized for mobile viewing

### 5. Time Range Selection
**Filename**: `monitoring-time-ranges.png`
**Description**: Detailed chart with different time ranges
**Elements to capture**:
- Same modal as #3 but showing 7d time range selected
- Different X-axis labels showing dates
- Button group with "7d" highlighted

### 6. Configuration Options
**Filename**: `monitoring-config.png`
**Description**: Configuration file showing monitoring settings
**Elements to capture**:
- config.toml file open in editor
- `monitoring_enabled = true` line highlighted
- Or environment variable example

## Screenshot Guidelines

1. **Resolution**: Take screenshots at 1920x1080 or higher
2. **Format**: PNG format for clarity
3. **Annotations**: Add arrows or boxes to highlight key features if needed
4. **Data**: Ensure realistic-looking data in sparklines (not flat lines)
5. **Timing**: Take screenshots when metrics show interesting variations

## Where to Place Screenshots

1. Create directory: `/opt/ontree/ontree-node/docs/images/monitoring/`
2. Place all screenshots in this directory
3. Update README.md to reference screenshots:

```markdown
## System Monitoring

![Monitoring Dashboard](docs/images/monitoring/monitoring-dashboard.png)

OnTree includes a comprehensive system monitoring dashboard...

### Detailed View

![Detailed Chart](docs/images/monitoring/monitoring-detailed-chart.png)

Click any sparkline to see detailed historical data...
```

## Additional Documentation Screenshots (Optional)

1. **Network Metrics**: Show actual network upload/download rates when implemented
2. **Performance**: Show page load times in browser dev tools
3. **Error States**: What happens when no data is available
4. **Loading States**: HTMX update in progress indicators