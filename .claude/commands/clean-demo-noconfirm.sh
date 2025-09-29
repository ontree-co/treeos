#!/bin/bash

# TreeOS Demo Mode Cleanup Script (Non-interactive)
# Removes all local demo mode data without confirmation

set -e

echo "TreeOS Demo Mode Cleanup"
echo "========================"
echo ""
echo "Removing all demo mode data:"
echo "  - Application configurations (./apps/)"
echo "  - Shared data including Ollama models (./shared/)"
echo "  - Log files (./logs/)"
echo "  - Database (./ontree.db)"
echo ""
echo "Starting cleanup..."

# Remove directories
[ -d "./apps" ] && rm -rf ./apps && echo "✓ Removed ./apps/"
[ -d "./shared" ] && rm -rf ./shared && echo "✓ Removed ./shared/"
[ -d "./logs" ] && rm -rf ./logs && echo "✓ Removed ./logs/"

# Remove database
[ -f "./ontree.db" ] && rm -f ./ontree.db && echo "✓ Removed ./ontree.db"
[ -f "./ontree.db-shm" ] && rm -f ./ontree.db-shm && echo "✓ Removed ./ontree.db-shm"
[ -f "./ontree.db-wal" ] && rm -f ./ontree.db-wal && echo "✓ Removed ./ontree.db-wal"

echo ""
echo "Demo mode cleanup complete!"
echo ""
echo "To start fresh, run TreeOS with TREEOS_RUN_MODE=demo"