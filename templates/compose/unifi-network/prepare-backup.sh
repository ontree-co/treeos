#!/bin/bash

# Unifi Network Application - Backup Data Preparation Script
# This script prepares your existing backup data for TreeOS deployment

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if backup directory is provided
if [ "$#" -ne 1 ]; then
    echo -e "${RED}Usage: $0 <path-to-unifi-backup-directory>${NC}"
    echo "Example: $0 /path/to/unifi-backup"
    exit 1
fi

BACKUP_SOURCE="$1"
BACKUP_DEST="./backup-data/prepared"

echo -e "${GREEN}Unifi Network Application - Backup Data Preparation${NC}"
echo "=================================================="

# Verify source directory exists
if [ ! -d "$BACKUP_SOURCE" ]; then
    echo -e "${RED}Error: Backup source directory not found: $BACKUP_SOURCE${NC}"
    exit 1
fi

# Create destination directory
echo -e "${YELLOW}Creating backup destination directory...${NC}"
mkdir -p "$BACKUP_DEST"

# Function to copy and prepare data
prepare_data() {
    local source_dir="$1"
    local dest_name="$2"

    if [ -d "$BACKUP_SOURCE/$source_dir" ]; then
        echo -e "${GREEN}✓${NC} Found $source_dir - preparing for restoration..."

        # Create tar archive for easy transfer
        tar -czf "$BACKUP_DEST/${dest_name}.tar.gz" -C "$BACKUP_SOURCE" "$source_dir"

        # Calculate size
        SIZE=$(du -sh "$BACKUP_DEST/${dest_name}.tar.gz" | cut -f1)
        echo "  Archive created: ${dest_name}.tar.gz (${SIZE})"
    else
        echo -e "${YELLOW}⚠${NC} $source_dir not found - skipping"
    fi
}

# Prepare MongoDB data
echo ""
echo "Preparing MongoDB data..."
prepare_data "mongo-data-volume" "mongodb-backup"

# Prepare Unifi configuration
echo ""
echo "Preparing Unifi configuration..."
prepare_data "unifi-config-volume" "unifi-config-backup"

# Prepare Unifi app data (if exists)
echo ""
echo "Preparing Unifi application data..."
prepare_data "unifi-appdata" "unifi-appdata-backup"

# Create restoration script
echo ""
echo "Creating restoration script..."
cat > "$BACKUP_DEST/restore.sh" << 'EOF'
#!/bin/bash

# Unifi Network Application - Data Restoration Script
# Run this script on your TreeOS server after deploying the application

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

APP_NAME="unifi-network"
PROJECT_NAME="ontree-${APP_NAME}"

echo -e "${GREEN}Unifi Network Application - Data Restoration${NC}"
echo "============================================="

# Check if running on TreeOS server
if [ ! -d "/opt/ontree/apps" ]; then
    echo -e "${RED}Error: This script must be run on the TreeOS server${NC}"
    exit 1
fi

# Check if app is deployed
if [ ! -d "/opt/ontree/apps/${APP_NAME}" ]; then
    echo -e "${RED}Error: Unifi Network Application not deployed yet${NC}"
    echo "Please deploy the app first through TreeOS dashboard"
    exit 1
fi

cd "/opt/ontree/apps/${APP_NAME}"

# Stop containers
echo -e "${YELLOW}Stopping containers...${NC}"
docker-compose down

# Function to restore volume
restore_volume() {
    local archive="$1"
    local volume_name="$2"
    local source_dir="$3"

    if [ -f "$archive" ]; then
        echo -e "${GREEN}Restoring $volume_name...${NC}"

        # Get volume path
        VOLUME_PATH="/var/lib/docker/volumes/${PROJECT_NAME}_${volume_name}/_data"

        # Backup existing data (just in case)
        if [ -d "$VOLUME_PATH" ]; then
            sudo tar -czf "${volume_name}-backup-$(date +%Y%m%d-%H%M%S).tar.gz" -C "$VOLUME_PATH" .
        fi

        # Clear and restore
        sudo rm -rf "$VOLUME_PATH"/*
        sudo tar -xzf "$archive" -C /tmp/
        sudo mv "/tmp/$source_dir"/* "$VOLUME_PATH/"
        sudo rm -rf "/tmp/$source_dir"

        echo "  ✓ Restored successfully"
    else
        echo -e "${YELLOW}⚠ Archive $archive not found - skipping${NC}"
    fi
}

# Restore MongoDB data
restore_volume "mongodb-backup.tar.gz" "unifi_mongo_data" "mongo-data-volume"
sudo chown -R 999:999 "/var/lib/docker/volumes/${PROJECT_NAME}_unifi_mongo_data/_data"

# Restore Unifi config
restore_volume "unifi-config-backup.tar.gz" "unifi_config" "unifi-config-volume"

# Restore Unifi app data if exists
if [ -f "unifi-appdata-backup.tar.gz" ]; then
    restore_volume "unifi-appdata-backup.tar.gz" "unifi_config" "unifi-appdata"
fi

# Set proper permissions for Unifi data
PUID=$(grep PUID .env | cut -d= -f2)
PGID=$(grep PGID .env | cut -d= -f2)
sudo chown -R ${PUID:-1000}:${PGID:-1000} "/var/lib/docker/volumes/${PROJECT_NAME}_unifi_config/_data"

# Restart containers
echo -e "${YELLOW}Starting containers...${NC}"
docker-compose up -d

echo ""
echo -e "${GREEN}✓ Restoration complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Wait ~2 minutes for services to fully start"
echo "2. Access Unifi at: https://$(hostname -I | awk '{print $1}'):8443"
echo "3. Login with your existing credentials"
echo "4. Verify all devices and settings are restored"
echo ""
echo -e "${YELLOW}Note: You may need to re-adopt devices if the server IP has changed${NC}"
EOF

chmod +x "$BACKUP_DEST/restore.sh"

# Create deployment instructions
cat > "$BACKUP_DEST/DEPLOYMENT.md" << EOF
# Unifi Network Application - Deployment Instructions

## Files in this backup

- **mongodb-backup.tar.gz**: MongoDB database containing all Unifi data
- **unifi-config-backup.tar.gz**: Unifi application configuration
- **unifi-appdata-backup.tar.gz**: Additional Unifi application data (if present)
- **restore.sh**: Automated restoration script

## Deployment Steps

### 1. Transfer files to TreeOS server

\`\`\`bash
# From your local machine
scp -r prepared/* user@treeos-server:/tmp/unifi-restore/
\`\`\`

### 2. Deploy Unifi Network Application

1. Login to TreeOS Dashboard
2. Navigate to Apps > Add New App
3. Select "Unifi Network Application"
4. Configure settings (ensure MongoDB password is set)
5. Deploy the application

### 3. Run restoration

\`\`\`bash
# On TreeOS server
cd /tmp/unifi-restore
sudo ./restore.sh
\`\`\`

### 4. Verify restoration

- Access: https://your-server:8443
- Login with existing credentials
- Check all devices are visible
- Verify configurations are intact

## Important Notes

- **MongoDB Version**: Using 6.0.15 for compatibility
- **Passwords**: Your existing Unifi passwords are preserved
- **Device Adoption**: May need to re-inform devices if IP changed
- **Backup**: A backup is automatically created before restoration

## Troubleshooting

If restoration fails:
1. Check docker logs: \`docker-compose logs -f\`
2. Verify file permissions
3. Ensure sufficient disk space
4. Check MongoDB connectivity

EOF

# Summary
echo ""
echo -e "${GREEN}✅ Backup preparation complete!${NC}"
echo ""
echo "Prepared files location: $BACKUP_DEST"
echo ""
echo "Contents:"
ls -lh "$BACKUP_DEST"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Transfer the contents of $BACKUP_DEST to your TreeOS server"
echo "2. Deploy Unifi Network Application via TreeOS dashboard"
echo "3. Run the restore.sh script on the server"
echo ""
echo "See $BACKUP_DEST/DEPLOYMENT.md for detailed instructions"