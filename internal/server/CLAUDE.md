# Server Package Documentation

## Handler Updates for Compose-Based Metadata (2025-07-10)

### Overview
Updated all HTTP handlers to use docker-compose.yml files as the single source of truth for app metadata, removing dependency on the `deployed_apps` database table.

### Modified Handlers

#### handleAppDetail (handlers.go:320)
- Reads metadata from compose files using `yamlutil.ReadComposeMetadata()`
- Creates a DeployedApp-like structure for template compatibility
- No longer queries the database for app metadata

#### handleAppExpose (handlers.go:998) 
- Reads current metadata from compose file
- Accepts subdomain from form input
- Writes updated metadata (subdomain, is_exposed) back to compose file
- Extracts host_port from compose if not already set in metadata
- Uses file locking to prevent concurrent write issues

#### handleAppUnexpose (handlers.go:1120)
- Reads metadata from compose file to check exposure status
- Updates is_exposed to false in compose file
- Removes route from Caddy reverse proxy

#### handleAppStatusCheck (handlers.go:1353)
- Reads subdomain and exposure status from compose files
- No database queries for app metadata

#### handleAppStop (handlers.go:531)
- Additionally checks compose metadata when stopping containers
- If app was exposed, removes from Caddy and updates compose metadata

### App Creation Updates

#### createAppScaffold (app_create_handler.go:176)
- Extracts host port from docker-compose content
- Creates initial x-ontree metadata with:
  - subdomain (defaults to app name)
  - host_port (extracted from compose)
  - is_exposed (defaults to false)
- Writes metadata to compose file after creation

### Key Implementation Details

1. **Backward Compatibility**: All handlers maintain compatibility with existing templates by creating DeployedApp-like structures

2. **Error Handling**: If compose metadata cannot be read, handlers initialize with sensible defaults

3. **File Locking**: All write operations use file-level mutex locking implemented in yamlutil

4. **ID Generation**: Since database IDs are no longer available, handlers generate pseudo-IDs in format `app-{appName}` for Caddy route management

### Migration Notes

After running the migration script, the application will use compose files exclusively. The database table can be dropped once all instances are updated.

## Stale Operation Handling

The app detail page filters Docker operations to prevent showing spinners for stale operations:

1. **Query Filter**: Only considers operations created within the last 5 minutes
   - Prevents UI from showing "Waiting to start..." for old, abandoned operations
   - See `handleAppDetail` in handlers.go:376
   - Query includes: `AND created_at > datetime('now', '-5 minutes')`

2. **Worker Cleanup**: Background worker marks stale operations as failed
   - Runs every minute to clean up operations older than 5 minutes
   - Prevents accumulation of pending/in_progress operations
   - Started automatically in `Worker.Start()`
   - See `cleanupStaleOperations` in worker/worker.go:291-327

3. **Test Coverage**: 
   - Unit test: `TestStaleOperationHandling` in handlers_test.go
   - E2E test: `tests/e2e/check-stale-operations.js`

### Bug History
- **Issue**: Containers showing "Waiting to start..." spinner even when not created
- **Cause**: Old operations from days ago remained in pending status
- **Fix**: Time-based filtering + automatic cleanup of stale operations

## Template Requirements

### Base Template Data Structure

All handlers that render HTML templates MUST provide certain fields that the base template expects:

1. **Messages** ([]interface{}) - Required by base.html for displaying flash messages
   - Can be nil if no messages
   - Format: `map[string]interface{}{"Type": "success|danger|info", "Text": "message"}`

2. **User** (interface{}) - Current user object or nil if not authenticated

3. **CSRFToken** (string) - CSRF protection token (currently empty, reserved for future use)

### Using baseTemplateData

The preferred way to create template data is to use the `baseTemplateData` method:

```go
data := s.baseTemplateData(user)
// Add handler-specific fields
data["Apps"] = apps
data["Errors"] = errors
```

This ensures all required fields are present.

### Flash Messages

To display flash messages, use the session store:

```go
session, _ := s.sessionStore.Get(r, "ontree-session")
session.AddFlash("Operation successful!", "success")
session.Save(r, w)
```

Messages are automatically retrieved and formatted in `handleAppDetail` as an example.

### Template Execution

Always use ExecuteTemplate with the "base" template:

```go
tmpl := s.templates["template_name"]
tmpl.ExecuteTemplate(w, "base", data)
```

## Monitoring Dashboard Handlers (2025-07-10)

### Overview
Added new handlers and routing structure for the system monitoring dashboard feature. The monitoring system uses HTMX for real-time updates without full page refreshes.

### Routes Structure
- `/monitoring` - Main dashboard page
- `/monitoring/partials/cpu` - CPU usage card partial
- `/monitoring/partials/memory` - Memory usage card partial
- `/monitoring/partials/disk` - Disk usage card partial
- `/monitoring/partials/network` - Network usage card partial
- `/monitoring/charts/{metric}` - Detailed chart views (modal content)

### Implementation Details

#### handlers_monitoring.go
Created new file with all monitoring-related handlers:

1. **handleMonitoring**: Renders the main monitoring dashboard
   - Currently returns placeholder HTML
   - Will be updated to use proper template system

2. **routeMonitoring**: Routes /monitoring/* requests
   - Handles partial updates and chart requests
   - Similar pattern to routeApps

3. **Partial Handlers**: Return HTMX-compatible HTML fragments
   - Each returns a complete card with hx-get for polling
   - 5-second refresh interval configured
   - Placeholder SVG sparklines included

4. **handleMonitoringCharts**: Returns detailed charts
   - Extracts metric type from URL path
   - Will return larger SVG charts for modal display

### HTMX Integration
- Cards use `hx-get` for automatic polling
- `hx-trigger="every 5s"` for real-time updates
- `hx-swap="outerHTML"` to replace entire card
- Follows existing HTMX patterns from system vitals

### Next Steps
These handlers are ready for:
- Integration with real system metrics data
- SVG sparkline generation from historical data
- Proper template integration
- Bootstrap styling to match OnTree design

## Handler Patterns

### GET Handler Example
```go
func (s *Server) handleExample(w http.ResponseWriter, r *http.Request) {
    user := getUserFromContext(r.Context())
    
    data := s.baseTemplateData(user)
    data["PageSpecificData"] = someData
    
    tmpl := s.templates["example"]
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    tmpl.ExecuteTemplate(w, "base", data)
}
```

### POST Handler with Validation
```go
func (s *Server) handleExamplePost(w http.ResponseWriter, r *http.Request) {
    user := getUserFromContext(r.Context())
    
    if r.Method == "POST" {
        // Process form...
        var errors []string
        
        if len(errors) > 0 {
            data := s.baseTemplateData(user)
            data["Errors"] = errors
            data["FormData"] = formData
            
            tmpl := s.templates["example"]
            tmpl.ExecuteTemplate(w, "base", data)
            return
        }
        
        // Success - redirect with flash message
        session, _ := s.sessionStore.Get(r, "ontree-session")
        session.AddFlash("Success!", "success")
        session.Save(r, w)
        http.Redirect(w, r, "/success", http.StatusFound)
    }
}
```