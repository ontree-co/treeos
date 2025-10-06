#!/bin/bash

# TreeOS Production Mode Deep Cleanup Script (Non-interactive)
# Removes ALL production data including shared folders, Docker containers and images

set -e

# Check if running as root (required for /opt operations)
if [ "$EUID" -ne 0 ]; then
    echo "This script must be run with sudo to clean /opt/ontree"
    exit 1
fi

echo "TreeOS Production Mode DEEP Cleanup"
echo "===================================="
echo ""
echo "Removing ALL TreeOS data including:"
echo "  - Application configurations (/opt/ontree/apps/)"
echo "  - Shared data (/opt/ontree/shared/)"
echo "  - Ollama models (/opt/ontree/shared/ollama/)"
echo "  - Log files (/opt/ontree/logs/)"
echo "  - Database (/opt/ontree/ontree.db)"
echo "  - TreeOS binary (/opt/ontree/treeos)"
echo "  - All Docker containers starting with 'ontree-'"
echo "  - All associated Docker images"
echo "  - Docker image cache"
echo ""
echo "Starting deep cleanup..."

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
    echo "Cleaning up Docker containers and images..."

    # Stop and remove all containers starting with 'ontree-'
    echo "Stopping ontree-* containers..."
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

    # Get all images used by ontree-* containers before removing them
    echo ""
    echo "Collecting images used by ontree-* containers..."
    IMAGES_TO_REMOVE=""

    # First, get images from any remaining container configs
    CONTAINER_IMAGES=$(docker ps -a --format "{{.Image}}" --filter "name=^ontree-" 2>/dev/null | sort -u || true)
    if [ ! -z "$CONTAINER_IMAGES" ]; then
        IMAGES_TO_REMOVE="$CONTAINER_IMAGES"
    fi

    # Also check for images that might be tagged with ontree prefix
    TAGGED_IMAGES=$(docker images --format "{{.Repository}}:{{.Tag}}" | grep -i "ontree" || true)
    if [ ! -z "$TAGGED_IMAGES" ]; then
        if [ ! -z "$IMAGES_TO_REMOVE" ]; then
            IMAGES_TO_REMOVE="$IMAGES_TO_REMOVE"$'\n'"$TAGGED_IMAGES"
        else
            IMAGES_TO_REMOVE="$TAGGED_IMAGES"
        fi
    fi

    # Remove collected images
    if [ ! -z "$IMAGES_TO_REMOVE" ]; then
        echo "Removing associated images..."
        echo "$IMAGES_TO_REMOVE" | sort -u | while read image; do
            if [ ! -z "$image" ]; then
                echo "  - Removing image: $image"
                docker rmi -f "$image" 2>/dev/null || true
            fi
        done
        echo "✓ Removed associated images"
    else
        echo "  No associated images found"
    fi

    # Prune Docker system to clear image cache
    echo ""
    echo "Pruning Docker image cache..."
    docker system prune -af --volumes 2>/dev/null || true
    echo "✓ Cleared Docker image cache"

else
    echo ""
    echo "ℹ Docker not found - skipping container cleanup"
fi

# Complete removal of /opt/ontree directory
echo ""
echo "Removing /opt/ontree directory..."
if [ -d "/opt/ontree" ]; then
    rm -rf /opt/ontree
    echo "✓ Removed entire /opt/ontree directory"
else
    echo "ℹ /opt/ontree directory does not exist"
fi

echo ""
echo "Deep cleanup complete!"
echo "TreeOS has been completely removed from the system."
echo "All Docker containers and images have been cleaned."
echo ""
echo "To reinstall TreeOS:"
echo "  1. Download the latest release"
echo "  2. Run the setup script"
echo ""
echo "Note: Container images will be re-downloaded on next use."