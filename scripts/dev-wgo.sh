#!/usr/bin/env bash
# TreeOS Development with Hot Reload using wgo
# This script provides hot reloading for Go files and automatic rebuild on template/static changes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Determine quiet mode: default to true when LOG_LEVEL=error (override with QUIET)
if [ "${LOG_LEVEL,,}" == "error" ]; then
    QUIET="${QUIET:-true}"
else
    QUIET="${QUIET:-false}"
fi

# Default DEBUG aligns with LOG_LEVEL unless explicitly set
if [ -z "${DEBUG+x}" ]; then
    if [ "${LOG_LEVEL,,}" == "error" ]; then
        DEBUG="false"
    else
        DEBUG="true"
    fi
fi

say() {
    if [ "$QUIET" != "true" ]; then
        echo -e "$@"
    fi
}

say_plain() {
    if [ "$QUIET" != "true" ]; then
        echo "$@"
    fi
}

# Load .env file if it exists
if [ -f .env ]; then
    say "${YELLOW}📋 Loading .env file...${NC}"
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

say "${GREEN}🚀 Starting TreeOS Development Server with Hot Reload${NC}"
say "${YELLOW}Configuration:${NC}"
say_plain "  Port: $PORT"
say_plain "  Debug Mode: $DEBUG"
say_plain "  Apps Directory: $ONTREE_APPS_DIR"
say_plain "  Database: $DATABASE_PATH"
say ""

# Ensure directories exist
mkdir -p "$ONTREE_APPS_DIR" logs

    # Function to build assets
build_assets() {
    say "${YELLOW}📦 Building embedded assets...${NC}"
    make embed-assets
}

# Initial asset build
build_assets

# Start wgo with file watching
# Watch Go files for backend changes
# Watch templates and static files for frontend changes
say "${GREEN}👁️  Starting file watcher with wgo...${NC}"
say "${YELLOW}Watching for changes in:${NC}"
say_plain "  - *.go files (application code)"
say_plain "  - templates/** (HTML templates)"
say_plain "  - static/** (CSS, JS, images)"
say ""

# Export environment variables for the application
export DEBUG="$DEBUG"
export ONTREE_APPS_DIR="$ONTREE_APPS_DIR"
export DATABASE_PATH="$DATABASE_PATH"
# Keep the original LISTEN_ADDR format from .env if it exists, otherwise use :PORT
if [ -z "$LISTEN_ADDR" ]; then
    export LISTEN_ADDR=":$PORT"
fi
export TREEOS_RUN_MODE="demo"
export QUIET="$QUIET"

build_url() {
    local addr="$1"
    case "$addr" in
        http://*|https://*)
            echo "$addr"
            ;;
        :*)
            echo "http://localhost$addr"
            ;;
        *:*)
            echo "http://$addr"
            ;;
        *)
            echo "http://$addr"
            ;;
    esac
}

SERVER_URL="$(build_url "$LISTEN_ADDR")"

# Run wgo with multiple file watchers
# Pass environment to the subshell
# IMPORTANT: Exclude internal/embeds to prevent infinite loop when assets are copied
wgo -file=".go" -file=".html" -file=".css" -file=".js" \
    -xdir="vendor" -xdir="node_modules" -xdir="documentation" -xdir="tests" -xdir="build" \
    -xdir=".git" -xdir="logs" -xdir="apps" -xdir="internal/embeds" \
    -debounce="500ms" \
    bash -c "
        if [ \"${QUIET}\" != \"true\" ]; then echo -e '${YELLOW}🔄 Changes detected, rebuilding...${NC}'; fi

        # Check if templates or static files changed
        if find templates static -newer build/treeos 2>/dev/null | grep -q .; then
            if [ \"${QUIET}\" != \"true\" ]; then echo -e '${YELLOW}📦 Rebuilding embedded assets...${NC}'; fi
            make embed-assets || exit 1
        fi

        # Build the application
        if [ \"${QUIET}\" != \"true\" ]; then echo -e '${YELLOW}🔨 Building application...${NC}'; fi
        go build -o build/treeos cmd/treeos/main.go || exit 1

        if [ \"${QUIET}\" != \"true\" ]; then echo -e \"${GREEN}✅ Build complete, starting server on ${SERVER_URL}...${NC}\"; fi
        if [ \"${QUIET}\" != \"true\" ]; then echo '----------------------------------------'; fi

        # Run the application with environment variables
        DEBUG='${DEBUG}' \
        ONTREE_APPS_DIR='${ONTREE_APPS_DIR}' \
        DATABASE_PATH='${DATABASE_PATH}' \
        LISTEN_ADDR='${LISTEN_ADDR}' \
        TREEOS_RUN_MODE='${TREEOS_RUN_MODE}' \
        QUIET='${QUIET}' \
        ./build/treeos
    "
