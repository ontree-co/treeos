# Worker Package

This package implements a background worker system for processing long-running Docker operations asynchronously.

## Overview

The worker system uses goroutines and channels to process Docker operations (start, recreate) in the background while providing real-time progress updates.

## Key Components

- **Worker**: Main struct that manages the worker pool and job queue
- **Job Queue**: Channel-based queue for operation IDs
- **Worker Pool**: Configurable number of goroutines processing operations

## Operations Supported

- **Start Container**: Creates and starts a new container, including image pulls
- **Recreate Container**: Stops, removes, and creates a new container with latest images

## Progress Tracking

Operations update their status in the database:
- `pending`: Operation queued but not started
- `in_progress`: Operation actively being processed
- `completed`: Operation finished successfully
- `failed`: Operation encountered an error

Progress percentages and messages are updated throughout the operation for UI feedback.