# Migration Package

This package contains database migration utilities for OnTree.

## migrate_to_compose.go

Handles the migration of app metadata from the `deployed_apps` database table to the `x-ontree` extension field in docker-compose.yml files.

### Usage

Run the migration command:
```bash
./ontree-server migrate-to-compose
```

### What it does:

1. Creates a timestamped backup directory in the apps folder
2. Reads all records from the `deployed_apps` table
3. For each app:
   - Backs up the original docker-compose.yml
   - Adds/updates the `x-ontree` section with metadata (subdomain, host_port, is_exposed)
   - Preserves all existing YAML formatting and comments
4. Provides detailed logging of the migration progress
5. Reports success/failure statistics

### Error Handling:

- Skips apps where the directory or docker-compose.yml doesn't exist
- Continues processing remaining apps if one fails
- Creates backups before modifying any files
- Returns error if any apps fail to migrate (but completes all possible migrations)

### Post-Migration:

After successful migration, the `deployed_apps` table can be safely removed as all data is now stored in the compose files.