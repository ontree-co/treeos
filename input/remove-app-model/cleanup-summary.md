# DeployedApp Cleanup Summary

## Files with References to Remove

### 1. `/opt/ontree/treeos/internal/database/models.go`
- **Status**: ✅ ALREADY REMOVED - The DeployedApp struct has been removed from this file
- No further action needed

### 2. `/opt/ontree/treeos/internal/server/server.go`
- **Location**: `syncExposedApps()` function (lines 490-510)
- **Action**: Remove or update the entire function as it queries the `deployed_apps` table
- References:
  - Line 494: SQL query from `deployed_apps` table
  - Line 508: `var app database.DeployedApp`

### 3. `/opt/ontree/treeos/internal/server/handlers.go`
- **Location**: In `handleAppDetail` function
- **Action**: Remove the DeployedApp struct usage for template compatibility
- The code creates a DeployedApp-like structure for template compatibility but should be refactored

### 4. `/opt/ontree/treeos/internal/migration/migrate_to_compose.go`
- **Action**: This file can potentially be removed entirely after migration is complete
- Contains:
  - `getAllDeployedApps()` function that queries `deployed_apps` table
  - References to `database.DeployedApp` throughout

### 5. Migration Files (No action needed - these are historical)
- `/opt/ontree/treeos/migrations/002_add_deployed_apps.sql` - Creates the table
- `/opt/ontree/treeos/migrations/004_drop_deployed_apps.sql` - Drops the table

## Documentation Files (No action needed - these document the changes)
- Various CLAUDE.md files documenting the migration
- Input specification files in `/opt/ontree/treeos/input/`

## Summary
The main cleanup tasks are:
1. Remove DeployedApp struct from models.go
2. Remove or refactor syncExposedApps() in server.go
3. Update handleAppDetail in handlers.go to not use DeployedApp
4. Consider removing the migration package after ensuring migrations are complete