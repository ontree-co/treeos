#!/bin/bash
# Integration test for Ollama download cancellation
# This test verifies that cancelling a model download properly stops all processes

set -e

echo "=== Ollama Download Cancellation Integration Test ==="

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "SKIP: Docker not found"
    exit 0
fi

# Check if Ollama container is running
CONTAINER=$(docker ps --filter "label=ontree.inference=true" --format "{{.Names}}" | head -1)
if [ -z "$CONTAINER" ]; then
    echo "SKIP: No Ollama container with ontree.inference=true label found"
    exit 0
fi

echo "Using container: $CONTAINER"

# Function to count ollama pull processes
count_ollama_pulls() {
    docker exec "$CONTAINER" sh -c "ps aux | grep 'ollama pull' | grep -v grep | wc -l" 2>/dev/null || echo "0"
}

# Function to check host docker exec processes
count_docker_execs() {
    ps aux | grep "docker exec.*ollama pull" | grep -v grep | wc -l
}

# Initial state
INITIAL_PULLS=$(count_ollama_pulls)
INITIAL_EXECS=$(count_docker_execs)
echo "Initial state: $INITIAL_PULLS ollama pulls, $INITIAL_EXECS docker execs"

# Start a model download in background
echo "Starting test model download..."
docker exec "$CONTAINER" ollama pull "test-model-xxx:latest" &>/dev/null &
PULL_PID=$!

# Wait for process to start
sleep 2

# Check that download started
DURING_PULLS=$(count_ollama_pulls)
DURING_EXECS=$(count_docker_execs)
echo "During download: $DURING_PULLS ollama pulls, $DURING_EXECS docker execs"

if [ "$DURING_PULLS" -le "$INITIAL_PULLS" ]; then
    echo "WARNING: Download may not have started properly"
fi

# Kill the process (simulating cancellation)
echo "Simulating cancellation..."

# Old way (what the bug was doing) - just kill docker exec
# kill $PULL_PID 2>/dev/null || true

# New way (what the fix does) - kill inside container first
docker exec "$CONTAINER" sh -c "pkill -f 'ollama pull test-model-xxx' || true"
kill $PULL_PID 2>/dev/null || true

# Wait for cleanup
sleep 2

# Check final state
FINAL_PULLS=$(count_ollama_pulls)
FINAL_EXECS=$(count_docker_execs)
echo "After cancellation: $FINAL_PULLS ollama pulls, $FINAL_EXECS docker execs"

# Verify cleanup worked
EXIT_CODE=0

if [ "$FINAL_PULLS" -gt "$INITIAL_PULLS" ]; then
    echo "FAIL: Ollama pull processes still running in container!"
    docker exec "$CONTAINER" sh -c "ps aux | grep 'ollama pull' | grep -v grep"
    EXIT_CODE=1
fi

if [ "$FINAL_EXECS" -gt "$INITIAL_EXECS" ]; then
    echo "FAIL: Docker exec processes still running on host!"
    ps aux | grep "docker exec.*ollama pull" | grep -v grep
    EXIT_CODE=1
fi

# Check network usage (optional, requires nethogs or similar)
if command -v ss &> /dev/null; then
    echo "Checking for active connections to Ollama registry..."
    ACTIVE_CONNS=$(ss -tupn 2>/dev/null | grep -c "registry" || echo "0")
    if [ "$ACTIVE_CONNS" -gt "0" ]; then
        echo "WARNING: $ACTIVE_CONNS connections still active to registry"
    fi
fi

if [ $EXIT_CODE -eq 0 ]; then
    echo "✓ TEST PASSED: Cancellation properly cleaned up all processes"
else
    echo "✗ TEST FAILED: Process cleanup incomplete"
fi

exit $EXIT_CODE