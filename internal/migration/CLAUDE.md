# Migration Package

This package contains migration tools for OnTree application data and configurations.

## Available Migrations

### 1. Database to Compose Migration (`migrate_to_compose.go`)

Migrates app metadata from the old `deployed_apps` database table to the x-ontree section in docker-compose.yml files.

**Usage**: `./ontree-server migrate-to-compose`

**What it does**:
1. Creates a timestamped backup directory in the apps folder
2. Reads all records from the `deployed_apps` table
3. For each app:
   - Backs up the original docker-compose.yml
   - Adds/updates the `x-ontree` section with metadata (subdomain, host_port, is_exposed)
   - Preserves all existing YAML formatting and comments
4. Provides detailed logging of the migration progress
5. Reports success/failure statistics

**Error Handling**:
- Skips apps where the directory or docker-compose.yml doesn't exist
- Continues processing remaining apps if one fails
- Creates backups before modifying any files
- Returns error if any apps fail to migrate (but completes all possible migrations)

**Post-Migration**: After successful migration, the `deployed_apps` table can be safely removed as all data is now stored in the compose files.

### 2. Single-to-Multi Service Migration (`migrate_single_to_multi.go`)

Converts legacy single-container apps to the new multi-service format required by the Docker Compose integration.

**Usage**: `./ontree-server migrate-single-to-multi [app1 app2 ...]`

**What it does**:
1. Discovers containers with legacy naming pattern: `ontree-{appName}`
2. For each legacy container:
   - Creates app directory structure: `/opt/ontree/apps/{appName}/`
   - Creates mount directory: `/opt/ontree/apps/mount/{appName}/app/`
   - Inspects container to extract full configuration
   - Generates docker-compose.yml with all settings preserved
   - Creates .env file for sensitive environment variables
   - Renames container to new format: `ontree-{appName}-app-1`
3. Safely handles running containers (stops, renames, restarts)

**Generated Compose File**:
- Version: 3.8
- Service name: "app" (default for single-container migrations)
- Includes: image, ports, environment, volumes, restart policy
- Adds x-ontree metadata for migration tracking

**Safety Features**:
- Skips apps that already have docker-compose.yml
- Non-destructive: preserves all data and volumes
- Updates volume paths to new mount directory structure
- Filters system environment variables (PATH, HOME, etc.)
- Extracts secrets (KEY, PASSWORD, TOKEN) to .env file

**Error Handling**:
- Continues with remaining apps if one fails
- Detailed logging for troubleshooting
- Can be run multiple times safely (idempotent)

## Testing

Both migration tools have comprehensive unit tests:
- `migrate_single_to_multi_test.go` - Tests compose generation, env filtering, and utility functions

Run tests with: `go test ./internal/migration/...`