# App Backup and Restoration Guide

This document describes how to restore app data from backups in TreeOS, with specific focus on Readeck as a concrete example.

## Overview

TreeOS apps store their data in Docker volumes. When restoring from a backup, you need to understand the app's internal directory structure to correctly map the backup data into the volume.

## Readeck Restoration (Concrete Example)

### Understanding Readeck's Structure

Readeck uses a `config.toml` file that defines its data locations:

```toml
[main]
data_directory = "data"

[database]
source = "sqlite3:data/db.sqlite3"
```

This means:
- The container mounts the volume at `/readeck`
- Readeck looks for the database at `/readeck/data/db.sqlite3`
- The `config.toml` must be at `/readeck/config.toml`

### Key Locations

- **Container mount point**: `/readeck`
- **Docker volume**: `ontree-readeck_readeck_data`
- **Volume host path**: `/var/lib/docker/volumes/ontree-readeck_readeck_data/_data`
- **App directory**: `/opt/ontree/apps/readeck`

### Restoration Steps

1. **Stop the container:**
   ```bash
   cd /opt/ontree/apps/readeck
   sudo docker compose stop
   ```

2. **Backup current data (safety measure):**
   ```bash
   sudo tar -czf /home/ontree/readeck-current-backup-$(date +%Y%m%d-%H%M%S).tar.gz \
     -C /var/lib/docker/volumes/ontree-readeck_readeck_data/_data .
   ```

3. **Clear the volume:**
   ```bash
   sudo rm -rf /var/lib/docker/volumes/ontree-readeck_readeck_data/_data/*
   ```

4. **Extract backup with correct structure:**
   ```bash
   sudo tar -xzf /path/to/readeck-backup.tar.gz \
     -C /var/lib/docker/volumes/ontree-readeck_readeck_data/_data \
     --strip-components=1
   ```

   **CRITICAL**: The `--strip-components` value depends on your backup structure:
   - If backup contains `data/data/db.sqlite3`, use `--strip-components=1`
   - If backup contains `db.sqlite3` at root, use `--strip-components=0`

   Verify the result should be:
   ```
   /var/lib/docker/volumes/ontree-readeck_readeck_data/_data/
   ├── config.toml
   ├── data/
   │   └── db.sqlite3
   └── bookmarks/
   ```

5. **Fix permissions:**
   ```bash
   sudo chown -R 1000:1000 /var/lib/docker/volumes/ontree-readeck_readeck_data/_data
   ```

6. **Restart the container:**
   ```bash
   cd /opt/ontree/apps/readeck
   sudo docker compose start
   ```

### Verification

After restoration, check that:
1. The app doesn't show onboarding/registration screens
2. You can log in with your existing credentials
3. Your data (bookmarks, etc.) is present

### Common Issues

#### App Shows Onboarding After Restore

**Cause**: The directory structure doesn't match what the app expects. The container created a new database instead of using the restored one.

**Solution**:
1. Check what the `config.toml` specifies for `data_directory` and `database.source`
2. Verify the extracted files match that structure
3. Use the correct `--strip-components` value when extracting

**Debug commands**:
```bash
# Check actual structure
sudo ls -la /var/lib/docker/volumes/ontree-readeck_readeck_data/_data/

# Check what container sees
docker exec ontree-readeck-readeck-1 ls -la /readeck/

# Check config
sudo cat /var/lib/docker/volumes/ontree-readeck_readeck_data/_data/config.toml
```

#### Permission Errors

**Cause**: The restored files have wrong ownership.

**Solution**: Run the chown command from step 5. Readeck runs as UID 1000.

## General Principles for Other Apps

When restoring any app:

1. **Inspect the backup first:**
   ```bash
   tar -tzf backup.tar.gz | head -20
   ```

2. **Find the Docker volume:**
   ```bash
   docker volume ls | grep app-name
   docker volume inspect volume-name --format '{{.Mountpoint}}'
   ```

3. **Check app's configuration** to understand where it expects data:
   - Look for config files in the backup
   - Check the app's docker-compose.yml for mount points
   - Examine the app's documentation

4. **Test with the container running** to see what it's looking for:
   ```bash
   docker logs container-name
   docker exec container-name ls -la /mount-point/
   ```

5. **Always backup before restore** - don't overwrite working data without a safety net

## Future Improvements

Consider implementing in TreeOS:
- Automated backup/restore commands: `treeos backup app-name` and `treeos restore app-name backup.tar.gz`
- Validation of backup structure before restoration
- Automatic detection of required `--strip-components` value
- Pre/post restore hooks for apps to handle migrations
- Backup metadata file that stores structure information
