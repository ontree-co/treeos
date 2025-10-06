# Agent Feature Removal Plan

## Overview
This document outlines the complete plan to remove agent-specific features from TreeOS while preserving the generic LLM configuration infrastructure that can be reused for other purposes.

## Key Principle
- **KEEP**: Generic LLM configuration (API key, URL, model selection, test connection)
- **REMOVE**: Agent-specific features (chat, status updates, agent operations)
- **RENAME**: All "agent_llm_*" references to "llm_*"

## Database Changes

### Tables to Remove
1. `agent_chat_messages` - Entire table can be dropped
2. `agent_operations` - Entire table can be dropped

### Settings Table Changes
Rename columns (migration required):
- `agent_llm_api_key` → `llm_api_key`
- `agent_llm_api_url` → `llm_api_url`
- `agent_llm_model` → `llm_model`

## Frontend Changes

### 1. App Detail Page (`templates/dashboard/app_detail.html`)

#### Remove Agent Status Card (Lines 424-473)
```html
<!-- DELETE THIS ENTIRE BLOCK -->
<div class="card mb-3">
    <div class="card-header">
        <i class="bi bi-robot"></i> Agent Status Updates
    </div>
    ...
</div>
```

#### Remove Chat Button (Lines 407-410)
```html
<!-- DELETE THIS BUTTON -->
<button type="button" class="btn btn-sm btn-outline-primary"
        onclick="showAgentChatModal('{{.App.Name}}')">
    <i class="bi bi-chat-dots"></i> Chat with Agent
</button>
```

#### Remove JavaScript Functions (Lines 1428-1574)
Delete these functions entirely:
- `showAgentChatModal(appName)`
- `sendAgentMessage()`
- `updateAgentOperationStatus(operationId, status)`
- `addMessageToChat(message, isUser)`
- `scrollChatToBottom()`
- `pollAgentOperations(appName)`
- `clearAgentPolling()`

#### Remove Chat Modal HTML (Lines 1575-1626)
```html
<!-- DELETE THIS ENTIRE MODAL -->
<div class="modal fade" id="agentChatModal" ...>
    ...
</div>
```

#### Clean up Agent Status Polling
Remove from Lines 1376-1408:
- All references to `agentPollingInterval`
- Calls to `pollAgentOperations()`
- Calls to `clearAgentPolling()`

### 2. Settings Page (`templates/dashboard/settings.html`)

#### Rename Section (Lines 192-313)
Change:
- Card header from "LLM Configuration" to "AI Configuration" or keep as "LLM Configuration"
- All JavaScript variable names from `agent_llm_*` to `llm_*`
- Form field names to match new database column names

Update JavaScript functions:
```javascript
// Line 358-403: Update testAgentConnection()
function testLLMConnection() {  // Rename function
    // Update all internal references from agent_llm_* to llm_*
}

// Line 404-431: Update saveAgentSettings()
function saveLLMSettings() {  // Rename function
    // Update all internal references
    // Update endpoint from /api/agent/settings to /api/llm/settings
}
```

## Backend Changes

### 1. Remove Handler Files
Delete entirely:
- `internal/server/handlers_agent.go`
- `internal/server/handlers_agent_test.go`

### 2. Update Settings Handler (`internal/server/handlers.go`)

#### Update handleSettings (Lines ~2300-2400)
- Remove agent operations query
- Update template data keys from `AgentLLM*` to `LLM*`

#### Update handleSettingsSave (Lines ~2450-2550)
- Change form field parsing from `agent_llm_*` to `llm_*`
- Update database column names in UPDATE query

### 3. Update Routes (`internal/server/routes.go`)

Remove agent-specific routes:
```go
// Remove these routes
router.HandleFunc("/api/agent/test", s.handleAgentTest).Methods("POST")
router.HandleFunc("/api/agent/chat", s.handleAgentChat).Methods("POST")
router.HandleFunc("/api/agent/operations/{id}/status", s.handleAgentOperationStatus).Methods("PUT")
```

Add/Keep LLM routes:
```go
// Add or rename to
router.HandleFunc("/api/llm/test", s.handleLLMTest).Methods("POST")
router.HandleFunc("/api/llm/settings", s.handleLLMSettings).Methods("POST")
```

### 4. Create New LLM Test Handler
Create simplified version in `handlers.go`:
```go
func (s *Server) handleLLMTest(w http.ResponseWriter, r *http.Request) {
    // Parse JSON request with api_key, api_url, model
    // Test connection to LLM API
    // Return success/error response
    // No agent operations, just connection test
}
```

### 5. Update Database Schema (`internal/database/schema.sql`)

Remove tables:
```sql
-- Remove these tables
DROP TABLE IF EXISTS agent_chat_messages;
DROP TABLE IF EXISTS agent_operations;
```

Update settings table:
```sql
-- Rename columns
ALTER TABLE settings RENAME COLUMN agent_llm_api_key TO llm_api_key;
ALTER TABLE settings RENAME COLUMN agent_llm_api_url TO llm_api_url;
ALTER TABLE settings RENAME COLUMN agent_llm_model TO llm_model;
```

### 6. Update Database Migrations (`internal/database/migrations/`)

Create new migration file:
```sql
-- 004_remove_agent_features.sql
DROP TABLE IF EXISTS agent_chat_messages;
DROP TABLE IF EXISTS agent_operations;

ALTER TABLE settings RENAME COLUMN agent_llm_api_key TO llm_api_key;
ALTER TABLE settings RENAME COLUMN agent_llm_api_url TO llm_api_url;
ALTER TABLE settings RENAME COLUMN agent_llm_model TO llm_model;
```

### 7. Update Server Struct (`internal/server/server.go`)

Remove agent-related fields if any exist:
- Remove agent client
- Remove agent service references

### 8. Remove Agent Package
Delete entirely if exists:
- `internal/agent/` directory and all contents

## Testing Requirements

### 1. Update Tests
- Remove `internal/server/handlers_agent_test.go`
- Update settings tests to use new column names
- Update API endpoint tests

### 2. Manual Testing Checklist
- [ ] Settings page loads correctly
- [ ] LLM configuration can be saved
- [ ] Test connection works with valid API credentials
- [ ] App detail page loads without errors
- [ ] No JavaScript console errors
- [ ] Database migrations run successfully
- [ ] No references to "agent" in user-facing UI

## Implementation Order

1. **Database Migration**
   - Create and run migration to rename columns
   - Drop agent-specific tables

2. **Backend Changes**
   - Update handlers and routes
   - Remove agent-specific files
   - Update settings handler

3. **Frontend Changes**
   - Remove agent UI components from app_detail.html
   - Update settings.html to use new names
   - Test JavaScript functionality

4. **Cleanup**
   - Remove unused imports
   - Update any documentation
   - Run tests and linting

## Verification Steps

1. Search for remaining "agent" references:
   ```bash
   grep -r "agent" --include="*.go" --include="*.html" --include="*.js" internal/ templates/
   ```

2. Check for orphaned JavaScript functions

3. Verify database schema matches expectations

4. Test full user flow:
   - Configure LLM settings
   - Test connection
   - View app details
   - Ensure no broken UI elements

## Files to Modify

### High Priority (Core Functionality)
1. `templates/dashboard/app_detail.html` - Remove agent UI (Lines 424-473, 407-410, 1428-1574, 1575-1626)
2. `templates/dashboard/settings.html` - Rename variables (Lines 192-313, 358-431)
3. `internal/server/handlers.go` - Update settings handlers (Lines ~2300-2550)
4. `internal/server/routes.go` - Update API routes
5. `internal/database/schema.sql` - Update schema

### Medium Priority (Cleanup)
1. `internal/server/handlers_agent.go` - Delete file
2. `internal/server/handlers_agent_test.go` - Delete file
3. `internal/agent/` - Delete directory if exists

### Low Priority (Documentation)
1. `internal/server/CLAUDE.md` - Update if references exist
2. Any API documentation

## Notes for Implementation

- This is a breaking change - no backward compatibility needed per user request
- Focus on clean removal rather than deprecation
- The LLM configuration should remain generic and reusable
- Test thoroughly as agent features may be referenced in unexpected places
- Consider adding feature flags if gradual rollout is needed

## Post-Implementation

After implementation:
1. Run full test suite
2. Check for any remaining "agent" references in codebase
3. Update any user documentation
4. Verify database migrations work on fresh install
5. Test upgrade path from existing installation