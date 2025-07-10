# Worker Package

This package implements a background worker system for processing long-running Docker operations asynchronously.

## Overview

The worker system uses goroutines and channels to process Docker operations (start, recreate) in the background while providing real-time progress updates.

## Key Components

- **Worker**: Main struct that manages the worker pool and job queue
- **Job Queue**: Channel-based queue for operation IDs
- **Worker Pool**: Configurable number of goroutines processing operations
- **OperationLogger**: Captures detailed logs for each operation

## Operations Supported

- **Start Container**: Creates and starts a new container, including image pulls
- **Recreate Container**: Stops, removes, and creates a new container with latest images
- **Update Image**: Checks for and pulls newer versions of Docker images

## Notable Exclusions

**Expose/Unexpose operations are NOT handled by the worker**. These operations are implemented synchronously in the HTTP handlers (`handleAppExpose` and `handleAppUnexpose`) because:
- They are typically fast operations (just Caddy API calls)
- They complete within reasonable HTTP request timeouts
- They already use compose files as the source of truth via yamlutil package

Note: The specification mentions `processExposeOperation` and `processUnexposeOperation` but these were never implemented.

## Progress Tracking

Operations update their status in the database:
- `pending`: Operation queued but not started
- `in_progress`: Operation actively being processed
- `completed`: Operation finished successfully
- `failed`: Operation encountered an error

Progress percentages and messages are updated throughout the operation for UI feedback.

## Operation Logging

The worker now includes comprehensive logging functionality:

### OperationLogger (logger.go)

- Logs all operation activities to `docker_operation_logs` table
- Log levels: DEBUG, INFO, WARNING, ERROR
- Captures:
  - Operation lifecycle events
  - Docker API interactions
  - Equivalent Docker CLI commands
  - Progress updates
  - Errors with full context

### Log Storage

Logs are stored in the database with:
- Operation ID reference
- Timestamp
- Log level
- Message
- Optional JSON details (commands, API calls, etc.)

### Accessing Logs

- Logs can be retrieved via `/api/docker/operations/{id}/logs` endpoint
- UI displays logs in real-time during operations
- Logs are retained for debugging purposes

## Cleanup

- **Stale Operations**: Operations older than 5 minutes are automatically marked as failed
- **Log Retention**: Logs can be cleaned up using `CleanupOldLogs()` function