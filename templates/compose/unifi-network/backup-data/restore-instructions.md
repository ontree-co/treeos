# Unifi Network Application - Backup Restoration Guide

## Overview
This guide explains how to restore your existing Unifi backup data to a new TreeOS deployment.

## Prerequisites
- TreeOS instance with Unifi Network Application deployed
- Access to your backup data from `unifi-backup/` directory
- SSH/terminal access to your TreeOS server

## Backup Data Structure

Your backup contains three main components:
1. **MongoDB Data** (`mongo-data-volume/`)
2. **Unifi Application Data** (`unifi-appdata/`)
3. **Unifi Configuration** (`unifi-config-volume/`)

## Step-by-Step Restoration Process

### 1. Deploy the Unifi Network Application

```bash
# On your TreeOS server
cd /opt/ontree/apps/
mkdir unifi-network
cd unifi-network

# Copy the docker-compose.yml from template
cp /path/to/templates/compose/unifi-network.yml docker-compose.yml

# Copy and configure the .env file
cp /path/to/templates/compose/unifi-network/.env.template .env
nano .env  # Edit and set a secure MONGO_PASS
```

### 2. Start the containers initially

```bash
docker-compose up -d
# Wait for containers to fully initialize (about 1 minute)
docker-compose ps  # Verify both containers are running
```

### 3. Stop the containers for data restoration

```bash
docker-compose down
```

### 4. Restore MongoDB Data

```bash
# Find the MongoDB volume
docker volume ls | grep unifi_mongo_data

# Copy your backup MongoDB data
sudo rm -rf /var/lib/docker/volumes/ontree-unifi-network_unifi_mongo_data/_data/*
sudo cp -r /path/to/unifi-backup/mongo-data-volume/* /var/lib/docker/volumes/ontree-unifi-network_unifi_mongo_data/_data/
sudo chown -R 999:999 /var/lib/docker/volumes/ontree-unifi-network_unifi_mongo_data/_data/
```

### 5. Restore Unifi Configuration

```bash
# Find the Unifi config volume
docker volume ls | grep unifi_config

# Copy your backup configuration
sudo rm -rf /var/lib/docker/volumes/ontree-unifi-network_unifi_config/_data/*
sudo cp -r /path/to/unifi-backup/unifi-config-volume/* /var/lib/docker/volumes/ontree-unifi-network_unifi_config/_data/

# If you have unifi-appdata, also copy it
sudo cp -r /path/to/unifi-backup/unifi-appdata/* /var/lib/docker/volumes/ontree-unifi-network_unifi_config/_data/

# Set proper permissions (adjust PUID/PGID from your .env file)
sudo chown -R 1000:1000 /var/lib/docker/volumes/ontree-unifi-network_unifi_config/_data/
```

### 6. Restart the Application

```bash
docker-compose up -d
docker-compose logs -f  # Monitor startup logs
```

### 7. Verify the Restoration

1. Access the Unifi Controller at: `https://your-server:8443`
2. Login with your existing credentials
3. Verify all your devices and configurations are present
4. Check that all adopted devices reconnect

## Important Notes

### Version Compatibility
- This template uses MongoDB 6.0.15 to maintain compatibility with your backup
- The Unifi Network Application version is pinned to 8.4.62 for stability
- If your backup is from a different version, you may need to adjust the image tags

### Security Considerations
- **NEVER** commit the `.env` file with real passwords to git
- Use strong passwords for MongoDB
- Consider using Docker secrets for production deployments
- Enable firewall rules to restrict access to Unifi ports

### Troubleshooting

#### MongoDB Connection Issues
```bash
# Check MongoDB logs
docker-compose logs mongo

# Test MongoDB connectivity
docker exec -it ontree-unifi-network-mongo-1 mongosh --eval "db.adminCommand('ping')"
```

#### Permission Issues
```bash
# Fix permissions for MongoDB
sudo chown -R 999:999 /var/lib/docker/volumes/ontree-unifi-network_unifi_mongo_data/_data/

# Fix permissions for Unifi (use PUID/PGID from .env)
sudo chown -R 1000:1000 /var/lib/docker/volumes/ontree-unifi-network_unifi_config/_data/
```

#### Port Conflicts
If ports are already in use, modify the port mappings in your `.env` file:
```bash
UNIFI_HTTPS_PORT=8444  # Change from 8443
UNIFI_HTTP_PORT=8081   # Change from 8080
```

### Backup Strategy

To create future backups:
```bash
# Backup script
#!/bin/bash
BACKUP_DIR="/backup/unifi-$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Stop containers
docker-compose down

# Backup volumes
docker run --rm -v ontree-unifi-network_unifi_mongo_data:/data -v $BACKUP_DIR:/backup alpine tar czf /backup/mongo-data.tar.gz -C /data .
docker run --rm -v ontree-unifi-network_unifi_config:/data -v $BACKUP_DIR:/backup alpine tar czf /backup/unifi-config.tar.gz -C /data .

# Restart containers
docker-compose up -d
```

## Support

For issues specific to:
- TreeOS integration: Check TreeOS documentation
- Unifi Network Application: https://community.ui.com/
- Docker/container issues: Check container logs with `docker-compose logs`