#!/bin/bash

# TreeOS Production Mode Cleanup Script (Non-interactive)
# Removes production data while PRESERVING shared folders

set -e

# Check if running as root (required for /opt operations)
if [ "$EUID" -ne 0 ]; then
    echo "This script must be run with sudo to clean /opt/ontree"
    exit 1
fi

echo "TreeOS Production Mode Cleanup"
echo "==============================="
echo ""
echo "Removing production data while preserving shared folders:"
echo "  REMOVING:"
echo "  - All containers starting with 'ontree-'"
echo "  - Application configurations (/opt/ontree/apps/)"
echo "  - Log files (/opt/ontree/logs/)"
echo "  - Database (/opt/ontree/ontree.db)"
echo "  - TreeOS binary (/opt/ontree/treeos)"
echo ""
echo "  PRESERVING:"
echo "  - Shared data (/opt/ontree/shared/)"
echo "  - Ollama models (/opt/ontree/shared/ollama/)"
echo ""
echo "Starting cleanup..."

# Stop TreeOS service if running
if command -v launchctl &> /dev/null; then
    # macOS
    launchctl unload /Library/LaunchDaemons/com.ontree.treeos.plist 2>/dev/null || true
    echo "✓ Stopped TreeOS service (macOS)"
elif command -v systemctl &> /dev/null; then
    # Linux with systemd
    systemctl stop treeos.service 2>/dev/null || true
    echo "✓ Stopped TreeOS service (systemd)"
fi

# Check if Docker is available
if command -v docker &> /dev/null; then
    echo ""
    echo "Stopping and removing ontree-* containers..."

    # Stop all containers starting with 'ontree-'
    CONTAINERS=$(docker ps -a --format "{{.Names}}" | grep "^ontree-" || true)
    if [ ! -z "$CONTAINERS" ]; then
        echo "$CONTAINERS" | while read container; do
            echo "  - Stopping and removing container: $container"
            docker stop "$container" 2>/dev/null || true
            docker rm -f "$container" 2>/dev/null || true
        done
        echo "✓ Removed all ontree-* containers"
    else
        echo "  No ontree-* containers found"
    fi
else
    echo ""
    echo "ℹ Docker not found - skipping container cleanup"
fi

echo ""
echo "Removing production files and directories..."

# Remove applications directory
[ -d "/opt/ontree/apps" ] && rm -rf /opt/ontree/apps && echo "✓ Removed /opt/ontree/apps/"

# Remove logs directory
[ -d "/opt/ontree/logs" ] && rm -rf /opt/ontree/logs && echo "✓ Removed /opt/ontree/logs/"

# Remove database
[ -f "/opt/ontree/ontree.db" ] && rm -f /opt/ontree/ontree.db && echo "✓ Removed /opt/ontree/ontree.db"
[ -f "/opt/ontree/ontree.db-shm" ] && rm -f /opt/ontree/ontree.db-shm && echo "✓ Removed /opt/ontree/ontree.db-shm"
[ -f "/opt/ontree/ontree.db-wal" ] && rm -f /opt/ontree/ontree.db-wal && echo "✓ Removed /opt/ontree/ontree.db-wal"

# Remove TreeOS binary
[ -f "/opt/ontree/treeos" ] && rm -f /opt/ontree/treeos && echo "✓ Removed /opt/ontree/treeos binary"

echo ""
echo "Production cleanup complete!"
echo "Shared data and Ollama models have been preserved."
echo ""
echo "To reinstall TreeOS, run the setup script."