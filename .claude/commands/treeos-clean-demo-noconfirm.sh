#!/bin/bash

# TreeOS Demo Mode Cleanup Script (Non-interactive)
# Removes local demo mode data without confirmation, preserving shared folder

set -e

echo "TreeOS Demo Mode Cleanup"
echo "========================"
echo ""
echo "Removing demo mode data:"
echo "  - All containers starting with 'ontree-'"
echo "  - Application configurations (./apps/)"
echo "  - Log files (./logs/)"
echo "  - Database (./ontree.db)"
echo ""
echo "Preserving:"
echo "  - Shared data including Ollama models (./shared/)"
echo ""
echo "Starting cleanup..."

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
echo "Removing demo mode files and directories..."

# Remove directories
[ -d "./apps" ] && rm -rf ./apps && echo "✓ Removed ./apps/"
[ -d "./logs" ] && rm -rf ./logs && echo "✓ Removed ./logs/"

# Remove database
[ -f "./ontree.db" ] && rm -f ./ontree.db && echo "✓ Removed ./ontree.db"
[ -f "./ontree.db-shm" ] && rm -f ./ontree.db-shm && echo "✓ Removed ./ontree.db-shm"
[ -f "./ontree.db-wal" ] && rm -f ./ontree.db-wal && echo "✓ Removed ./ontree.db-wal"

echo ""
echo "Demo mode cleanup complete!"
echo "Shared data and Ollama models have been preserved."
echo ""
echo "To start fresh, run TreeOS with TREEOS_RUN_MODE=demo"