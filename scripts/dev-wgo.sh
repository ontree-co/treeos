#!/usr/bin/env bash
# TreeOS Development with Hot Reload using wgo
# This script provides hot reloading for Go files and automatic rebuild on template/static changes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Load .env file if it exists
if [ -f .env ]; then
    echo -e "${YELLOW}📋 Loading .env file...${NC}"
    export $(cat .env | grep -v '^#' | xargs)
fi

# Parse LISTEN_ADDR to extract port if set
if [ ! -z "$LISTEN_ADDR" ]; then
    # Extract port from LISTEN_ADDR (e.g., ":4000" -> "4000")
    PORT="${LISTEN_ADDR#:}"
    PORT="${PORT:-3000}"
else
    PORT="${PORT:-3000}"
fi

# Default values (respecting .env overrides)
DEBUG="${DEBUG:-true}"
ONTREE_APPS_DIR="${ONTREE_APPS_DIR:-./apps}"
DATABASE_PATH="${DATABASE_PATH:-./ontree.db}"

echo -e "${GREEN}🚀 Starting TreeOS Development Server with Hot Reload${NC}"
echo -e "${YELLOW}Configuration:${NC}"
echo "  Port: $PORT"
echo "  Debug Mode: $DEBUG"
echo "  Apps Directory: $ONTREE_APPS_DIR"
echo "  Database: $DATABASE_PATH"
echo ""

# Ensure directories exist
mkdir -p "$ONTREE_APPS_DIR" logs

# Function to build assets
build_assets() {
    echo -e "${YELLOW}📦 Building embedded assets...${NC}"
    make embed-assets
}

# Initial asset build
build_assets

# Start wgo with file watching
# Watch Go files for backend changes
# Watch templates and static files for frontend changes
echo -e "${GREEN}👁️  Starting file watcher with wgo...${NC}"
echo -e "${YELLOW}Watching for changes in:${NC}"
echo "  - *.go files (application code)"
echo "  - templates/** (HTML templates)"
echo "  - static/** (CSS, JS, images)"
echo ""

# Export environment variables for the application
export DEBUG="$DEBUG"
export ONTREE_APPS_DIR="$ONTREE_APPS_DIR"
export DATABASE_PATH="$DATABASE_PATH"
# Keep the original LISTEN_ADDR format from .env if it exists, otherwise use :PORT
if [ -z "$LISTEN_ADDR" ]; then
    export LISTEN_ADDR=":$PORT"
fi
export TREEOS_RUN_MODE="demo"

# Run wgo with multiple file watchers
# Pass environment to the subshell
# IMPORTANT: Exclude internal/embeds to prevent infinite loop when assets are copied
wgo -file=".go" -file=".html" -file=".css" -file=".js" \
    -xdir="vendor" -xdir="node_modules" -xdir="documentation" -xdir="tests" -xdir="build" \
    -xdir=".git" -xdir="logs" -xdir="apps" -xdir="internal/embeds" \
    -debounce="500ms" \
    -verbose \
    bash -c "
        echo -e '${YELLOW}🔄 Changes detected, rebuilding...${NC}'

        # Check if templates or static files changed
        if find templates static -newer build/treeos 2>/dev/null | grep -q .; then
            echo -e '${YELLOW}📦 Rebuilding embedded assets...${NC}'
            make embed-assets || exit 1
        fi

        # Build the application
        echo -e '${YELLOW}🔨 Building application...${NC}'
        go build -o build/treeos cmd/treeos/main.go || exit 1

        echo -e '${GREEN}✅ Build complete, starting server on ${LISTEN_ADDR}...${NC}'
        echo '----------------------------------------'

        # Run the application with environment variables
        DEBUG='${DEBUG}' \
        ONTREE_APPS_DIR='${ONTREE_APPS_DIR}' \
        DATABASE_PATH='${DATABASE_PATH}' \
        LISTEN_ADDR='${LISTEN_ADDR}' \
        TREEOS_RUN_MODE='${TREEOS_RUN_MODE}' \
        ./build/treeos
    "