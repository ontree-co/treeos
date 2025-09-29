#!/bin/bash

# TreeOS Demo Mode Deep Cleanup Script (Non-interactive)
# Removes ALL demo mode data including Podman containers and images

set -e

echo "TreeOS Demo Mode DEEP Cleanup"
echo "============================="
echo ""
echo "Removing ALL demo mode data including:"
echo "  - Application configurations (./apps/)"
echo "  - Shared data including Ollama models (./shared/)"
echo "  - Log files (./logs/)"
echo "  - Database (./ontree.db)"
echo "  - All Podman containers starting with 'ontree-'"
echo "  - All associated Podman images"
echo ""
echo "Starting deep cleanup..."

# Check if Podman is available
if command -v podman &> /dev/null; then
    echo ""
    echo "Cleaning up Podman containers and images..."

    # Stop and remove all containers starting with 'ontree-'
    echo "Stopping ontree-* containers..."
    CONTAINERS=$(podman ps -a --format "{{.Names}}" | grep "^ontree-" || true)
    if [ ! -z "$CONTAINERS" ]; then
        echo "$CONTAINERS" | while read container; do
            echo "  - Stopping and removing container: $container"
            podman stop "$container" 2>/dev/null || true
            podman rm -f "$container" 2>/dev/null || true
        done
        echo "✓ Removed all ontree-* containers"
    else
        echo "  No ontree-* containers found"
    fi

    # Get all images used by ontree-* containers before removing them
    # This includes getting images from container inspect
    echo ""
    echo "Collecting images used by ontree-* containers..."
    IMAGES_TO_REMOVE=""

    # First, get images from any remaining container configs
    CONTAINER_IMAGES=$(podman ps -a --format "{{.Image}}" --filter "name=^ontree-" 2>/dev/null | sort -u || true)
    if [ ! -z "$CONTAINER_IMAGES" ]; then
        IMAGES_TO_REMOVE="$CONTAINER_IMAGES"
    fi

    # Also check for images that might be tagged with ontree prefix
    TAGGED_IMAGES=$(podman images --format "{{.Repository}}:{{.Tag}}" | grep -i "ontree" || true)
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
                podman rmi -f "$image" 2>/dev/null || true
            fi
        done
        echo "✓ Removed associated images"
    else
        echo "  No associated images found"
    fi

    # Prune any dangling images to ensure complete cleanup
    echo ""
    echo "Pruning dangling images and build cache..."
    podman image prune -af 2>/dev/null || true
    echo "✓ Pruned dangling images"

    # Clear build cache if available (Podman 3.0+)
    podman system prune -af --volumes 2>/dev/null || true
    echo "✓ Cleared Podman system cache"

else
    echo ""
    echo "ℹ Podman not found - skipping container cleanup"
fi

# Remove demo directories and files
echo ""
echo "Removing demo mode files and directories..."

# Remove directories
[ -d "./apps" ] && rm -rf ./apps && echo "✓ Removed ./apps/"
[ -d "./shared" ] && rm -rf ./shared && echo "✓ Removed ./shared/"
[ -d "./logs" ] && rm -rf ./logs && echo "✓ Removed ./logs/"

# Remove database
[ -f "./ontree.db" ] && rm -f ./ontree.db && echo "✓ Removed ./ontree.db"
[ -f "./ontree.db-shm" ] && rm -f ./ontree.db-shm && echo "✓ Removed ./ontree.db-shm"
[ -f "./ontree.db-wal" ] && rm -f ./ontree.db-wal && echo "✓ Removed ./ontree.db-wal"

echo ""
echo "Demo mode deep cleanup complete!"
echo "All demo data, containers, and images have been removed."
echo ""
echo "To start fresh, run TreeOS with TREEOS_RUN_MODE=demo"
echo "Note: Container images will be re-downloaded on next use."