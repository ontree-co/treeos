# Migration Specification: Remove deployed_apps Model

## Overview

The current implementation stores app-specific metadata (subdomain, port, exposure status) in a SQLite database table called `deployed_apps`. This creates a problem when multiple OnTree instances share the same Docker daemon but have separate databases - app metadata stored in one instance's database isn't available to other instances.

The solution is to move all app metadata from the database into the docker-compose.yml files themselves, making the compose files the single source of truth.

## Problem Statement

- Multiple OnTree instances on the same server share one Docker daemon
- Each instance has its own SQLite database
- App metadata in `deployed_apps` table is not shared between instances
- This causes "sql: no rows in result set" errors when accessing apps from different instances
- The original design intent was to store all configuration in docker-compose.yml files

## Current State

### deployed_apps Table Schema
```sql
CREATE TABLE deployed_apps (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    docker_compose TEXT,
    subdomain TEXT,
    host_port INTEGER,
    is_exposed BOOLEAN DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Data Currently Stored
- `id`: Unique identifier for the app
- `name`: App name (matches folder name)
- `subdomain`: Subdomain for Caddy reverse proxy
- `host_port`: Port to expose via reverse proxy
- `is_exposed`: Whether app is currently exposed via Caddy
- `docker_compose`: Copy of compose file (redundant)

## Proposed Solution

### Use docker-compose.yml Custom Extension Fields

Docker Compose supports custom extension fields prefixed with `x-`. We'll use `x-ontree` to store OnTree-specific metadata:

```yaml
version: '3'

# OnTree metadata - ignored by docker-compose but preserved
x-ontree:
  subdomain: myapp
  host_port: 8080
  is_exposed: true
  # Can add more fields as needed in the future
  
services:
  myapp:
    image: nginx:latest
    ports:
      - "8080:80"
```

### Benefits
1. **Single Source of Truth**: All app configuration in one file
2. **Portability**: Metadata travels with the app when copied/moved
3. **Multi-Instance Compatible**: All OnTree instances see the same data
4. **Backup Simplicity**: Just backup the apps directory
5. **Docker Compose Compatible**: Standard compose commands still work

## Implementation Steps

### Step 1: Create YAML Helper Functions
Create utility functions to safely read/write YAML while preserving formatting:
- `ReadComposeWithMetadata(path string) (*Compose, error)`
- `WriteComposeWithMetadata(path string, compose *Compose) error`
- `GetOnTreeMetadata(compose *Compose) *OnTreeMetadata`
- `SetOnTreeMetadata(compose *Compose, metadata *OnTreeMetadata)`

### Step 2: Data Migration Script
Create a migration script that:
1. Reads all records from `deployed_apps` table
2. For each app:
   - Parse existing docker-compose.yml
   - Add/update `x-ontree` section with metadata
   - Write updated compose file
   - Verify the update succeeded
3. Create backup of original compose files
4. Log migration progress and any errors

### Step 3: Update Handlers
Modify handlers to use compose files instead of database:

#### handleAppDetail
- Remove `deployed_apps` query
- Read metadata from compose file using helper functions
- Pass metadata to template

#### handleAppExpose
- Remove database inserts/updates
- Update compose file with new subdomain/port
- Create operation for background processing

#### handleAppUnexpose
- Remove database updates
- Update compose file to set `is_exposed: false`
- Create operation for background processing

### Step 4: Update Worker
Modify background operations:

#### processExposeOperation
- Read metadata from compose file
- Update Caddy configuration
- Update compose file with `is_exposed: true`

#### processUnexposeOperation
- Read metadata from compose file
- Remove from Caddy
- Update compose file with `is_exposed: false`

### Step 5: Update App Creation
When creating new apps:
- Include `x-ontree` section in generated compose file
- Remove database insert operations

### Step 6: Database Cleanup
After successful migration:
- Drop the `deployed_apps` table
- Remove model from `database/models.go`
- Remove table creation from `database/database.go`
- Add migration version tracking to prevent re-running

## Code Examples

### Compose Structure
```go
type ComposeFile struct {
    Version  string                    `yaml:"version"`
    Services map[string]interface{}    `yaml:"services"`
    XOnTree  *OnTreeMetadata          `yaml:"x-ontree,omitempty"`
    // Preserve other fields as map[string]interface{}
}

type OnTreeMetadata struct {
    Subdomain  string `yaml:"subdomain,omitempty"`
    HostPort   int    `yaml:"host_port,omitempty"`
    IsExposed  bool   `yaml:"is_exposed"`
}
```

### Reading Metadata
```go
func getAppMetadata(appPath string) (*OnTreeMetadata, error) {
    composePath := filepath.Join(appPath, "docker-compose.yml")
    data, err := os.ReadFile(composePath)
    if err != nil {
        return nil, err
    }
    
    var compose ComposeFile
    if err := yaml.Unmarshal(data, &compose); err != nil {
        return nil, err
    }
    
    if compose.XOnTree == nil {
        return &OnTreeMetadata{}, nil
    }
    
    return compose.XOnTree, nil
}
```

### Updating Metadata
```go
func updateAppMetadata(appPath string, metadata *OnTreeMetadata) error {
    composePath := filepath.Join(appPath, "docker-compose.yml")
    
    // Read existing file
    data, err := os.ReadFile(composePath)
    if err != nil {
        return err
    }
    
    // Parse as map to preserve all fields
    var compose map[string]interface{}
    if err := yaml.Unmarshal(data, &compose); err != nil {
        return err
    }
    
    // Update metadata
    compose["x-ontree"] = metadata
    
    // Write back with proper formatting
    output, err := yaml.Marshal(compose)
    if err != nil {
        return err
    }
    
    return os.WriteFile(composePath, output, 0644)
}
```

## Migration Process

### Pre-Migration Checklist
1. Backup database
2. Backup all docker-compose.yml files
3. Stop all OnTree instances
4. Ensure no active operations

### Migration Execution
1. Run migration script
2. Verify all apps have metadata in compose files
3. Test reading metadata from a few apps
4. Start one OnTree instance and test functionality
5. Start remaining instances

### Rollback Plan
1. Stop all OnTree instances
2. Restore docker-compose.yml files from backup
3. Restore database from backup
4. Revert code changes
5. Restart OnTree instances

## Testing Requirements

1. **Unit Tests**
   - YAML read/write functions
   - Metadata extraction and update
   - Migration logic

2. **Integration Tests**
   - Expose/unexpose operations
   - Multi-instance scenarios
   - Concurrent access handling

3. **Manual Testing**
   - Create new app with metadata
   - Expose/unexpose existing apps
   - Access from multiple instances
   - Verify Caddy integration

## Risks and Mitigations

### Risk: Concurrent File Access
**Mitigation**: Implement file locking during read/write operations

### Risk: YAML Corruption
**Mitigation**: 
- Validate YAML before writing
- Keep temporary backup during updates
- Use atomic file operations

### Risk: Migration Failure
**Mitigation**:
- Comprehensive pre-migration backup
- Incremental migration with verification
- Clear rollback procedures

### Risk: Performance Impact
**Mitigation**:
- Cache compose file reads where appropriate
- Batch updates when possible
- Monitor file I/O performance

## Future Considerations

1. **Additional Metadata**: The `x-ontree` section can be extended with:
   - Health check URLs
   - Backup schedules
   - Resource limits
   - Custom labels

2. **File Watching**: Implement file watching to detect external compose file changes

3. **Validation**: Add compose file validation to ensure required OnTree metadata

4. **Migration Tools**: Create CLI commands for:
   - Importing existing apps
   - Validating metadata
   - Bulk updates

## Timeline Estimate

1. Implementation: 2-3 days
2. Testing: 1-2 days
3. Migration tooling: 1 day
4. Documentation: 1 day

Total: ~1 week for complete migration