#!/bin/bash

echo "Starting Jaeger..."
docker-compose -f jaeger-compose.yml up -d

echo "Waiting for Jaeger to start..."
sleep 5

echo "Starting onTree server with Jaeger tracing..."
export OTEL_SERVICE_NAME=ontree-node
export OTEL_ENVIRONMENT=development
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318

./ontree-server &
SERVER_PID=$!

echo "Waiting for server to start..."
sleep 3

echo "Making test requests..."
# Test various endpoints
curl -s http://localhost:8083/ > /dev/null
curl -s http://localhost:8083/api/system-vitals > /dev/null

echo "Server PID: $SERVER_PID"
echo ""
echo "Jaeger UI is available at: http://localhost:16686"
echo "Look for 'ontree-node' service in the Jaeger UI"
echo ""
echo "Press Ctrl+C to stop the server and Jaeger"

# Wait for interrupt
wait $SERVER_PID