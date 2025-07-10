# Usage Graph Implementation Tickets

## Overview
These tickets implement the system monitoring dashboard as specified in specification.md. The feature will add real-time system metrics visualization with 24-hour historical sparklines using HTMX and server-generated SVG.

## Tickets

### TICKET-001: Enable Historical System Vitals Data Collection
**Priority**: High  
**Dependencies**: None  
**Estimated Effort**: 2-3 hours

**Description**: 
Activate the existing but unused system_vital_logs table to store historical system metrics. Currently, the system collects vitals but doesn't persist them.

**Tasks**:
- [ ] Modify `internal/server/handlers_system.go` to store vitals in the database when collected
- [ ] Implement data retention policy (keep last 7 days of data)
- [ ] Add database cleanup job to remove old metrics
- [ ] Update the vitals collection interval from 30s to 60s for efficiency

**Acceptance Criteria**:
- System vitals are stored in the database every minute
- Old data (>7 days) is automatically cleaned up
- Existing real-time display continues to work

---

### TICKET-002: Create Monitoring Routes and Handler Structure
**Priority**: High  
**Dependencies**: None  
**Estimated Effort**: 2 hours

**Description**:
Set up the routing structure and base handlers for the monitoring dashboard.

**Tasks**:
- [ ] Add `/monitoring` route to main router in `server.go`
- [ ] Create `handlers_monitoring.go` file with base handler functions
- [ ] Add routes for partial updates: `/monitoring/partials/cpu`, `/monitoring/partials/memory`, etc.
- [ ] Create placeholder handlers that return basic HTML

**Acceptance Criteria**:
- `/monitoring` route is accessible and returns a basic page
- All partial routes are defined and return placeholder content
- Follows existing routing patterns in the codebase

---

### TICKET-003: Implement SVG Sparkline Generation
**Priority**: High  
**Dependencies**: None  
**Estimated Effort**: 3-4 hours

**Description**:
Create a reusable SVG sparkline generator for visualizing time-series data.

**Tasks**:
- [ ] Create `internal/charts/sparkline.go` with SVG generation functions
- [ ] Implement `GenerateSparklineSVG(dataPoints []float64, width, height int) template.HTML`
- [ ] Add proper scaling and normalization for different metric types
- [ ] Include basic styling (stroke color, width)
- [ ] Write unit tests for sparkline generation

**Acceptance Criteria**:
- Function generates valid SVG markup
- Sparklines scale properly to fit data ranges
- SVG renders correctly in browsers
- Unit tests pass

---

### TICKET-004: Create Monitoring Dashboard Templates
**Priority**: Medium  
**Dependencies**: TICKET-002  
**Estimated Effort**: 3-4 hours

**Description**:
Build the HTMX-powered monitoring dashboard UI with a 2x2 grid layout.

**Tasks**:
- [ ] Create `templates/monitoring.html` with base dashboard layout
- [ ] Create partial templates: `_cpu_card.html`, `_memory_card.html`, `_disk_card.html`, `_network_card.html`
- [ ] Implement HTMX polling (every 5s) for each card
- [ ] Add Bootstrap responsive grid styling
- [ ] Style cards to match existing OnTree UI patterns

**Acceptance Criteria**:
- Dashboard displays 2x2 grid on desktop, stacks on mobile
- Each card shows metric name, current value, and sparkline placeholder
- HTMX polling is configured for auto-updates
- UI matches OnTree's existing design language

---

### TICKET-005: Implement Data Retrieval Functions
**Priority**: High  
**Dependencies**: TICKET-001  
**Estimated Effort**: 3-4 hours

**Description**:
Create database functions to retrieve historical metrics for sparkline generation.

**Tasks**:
- [ ] Add `GetMetricsLast24Hours(metricType string) ([]database.SystemVitalLog, error)` to database package
- [ ] Implement efficient queries with proper time filtering
- [ ] Add `GetLatestMetric(metricType string) (*database.SystemVitalLog, error)`
- [ ] Handle edge cases (no data, partial data)
- [ ] Add appropriate database indexes for performance

**Acceptance Criteria**:
- Functions return correct time-windowed data
- Queries are performant (<100ms)
- Handles missing data gracefully
- Database indexes improve query performance

---

### TICKET-006: Wire Up Monitoring Handlers with Real Data
**Priority**: High  
**Dependencies**: TICKET-003, TICKET-004, TICKET-005  
**Estimated Effort**: 4-5 hours

**Description**:
Connect the monitoring handlers to real system data and sparkline generation.

**Tasks**:
- [ ] Update CPU handler to fetch real CPU data and generate sparkline
- [ ] Update Memory handler with memory usage data
- [ ] Update Disk handler with disk usage (use "/" as default path)
- [ ] Implement Network handler with rate calculation from cumulative bytes
- [ ] Format current values appropriately (percentages, MB/s, etc.)

**Acceptance Criteria**:
- Each metric card displays real current values
- Sparklines show actual 24-hour historical trends
- Values update every 5 seconds via HTMX
- Network speeds show as rates, not cumulative bytes

---

### TICKET-007: Add Modal Detail View
**Priority**: Low  
**Dependencies**: TICKET-006  
**Estimated Effort**: 3-4 hours

**Description**:
Implement click-to-expand functionality for detailed metric views.

**Tasks**:
- [ ] Add modal container to base layout
- [ ] Make sparklines clickable with `hx-get` to load detailed chart
- [ ] Create `/monitoring/charts/{metric}` endpoints
- [ ] Generate larger SVG charts with axes and labels
- [ ] Add time range selector (1h, 6h, 24h, 7d)

**Acceptance Criteria**:
- Clicking a sparkline opens a modal with detailed chart
- Detailed charts include axes, labels, and grid lines
- Modal can be closed and reopened
- Time range selection updates the chart

---

### TICKET-008: Performance Optimization and Cleanup
**Priority**: Medium  
**Dependencies**: TICKET-006  
**Estimated Effort**: 2-3 hours

**Description**:
Optimize the monitoring system for production use.

**Tasks**:
- [ ] Implement caching for sparkline generation (5-minute cache)
- [ ] Batch database queries where possible
- [ ] Add connection pooling for database access
- [ ] Profile and optimize SVG generation
- [ ] Add monitoring feature flag in settings

**Acceptance Criteria**:
- Page load time <200ms
- Partial updates complete <100ms
- Database connection usage is efficient
- Feature can be toggled on/off

---

### TICKET-009: Documentation and Testing
**Priority**: Medium  
**Dependencies**: TICKET-006  
**Estimated Effort**: 2-3 hours

**Description**:
Document the monitoring feature and add integration tests.

**Tasks**:
- [ ] Update CLAUDE.md with monitoring feature documentation
- [ ] Add monitoring section to user documentation
- [ ] Write integration tests for monitoring endpoints
- [ ] Add example screenshots to documentation
- [ ] Document any new configuration options

**Acceptance Criteria**:
- CLAUDE.md includes monitoring feature details
- Integration tests cover all monitoring endpoints
- Documentation includes usage instructions
- Configuration options are documented

---

## Implementation Order

1. **Phase 1** (Foundation): TICKET-001, TICKET-002, TICKET-003
2. **Phase 2** (Core Features): TICKET-004, TICKET-005, TICKET-006
3. **Phase 3** (Enhancements): TICKET-007, TICKET-008
4. **Phase 4** (Polish): TICKET-009

## Technical Notes

- Leverage existing `gopsutil` integration for metrics collection
- Follow established HTMX patterns from system vitals implementation
- Use existing database connection and model patterns
- Maintain consistency with OnTree's UI/UX design language
- Consider mobile responsiveness throughout implementation