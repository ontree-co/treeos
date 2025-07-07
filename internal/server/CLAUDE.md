# Server Package Documentation

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