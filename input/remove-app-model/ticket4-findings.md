# Ticket 4 Investigation Findings

## Issue Discovered

Ticket 4 asks to "Modify the background worker's operations (`processExposeOperation`, `processUnexposeOperation`) to read metadata from the `docker-compose.yml` file instead of the database."

However, these functions **do not exist** in the codebase.

## Current State

1. **Expose/Unexpose are Synchronous**: The `handleAppExpose` and `handleAppUnexpose` functions in `internal/server/handlers.go` handle these operations synchronously within the HTTP request handler.

2. **No Background Operations**: Unlike container operations (start, stop, recreate), expose/unexpose do not:
   - Create entries in the `docker_operations` table
   - Use the worker system for background processing
   - Have operation types defined in the models

3. **Already Using Compose Files**: The current implementation already reads from and writes to compose files using the yamlutil package (completed in Ticket 3).

## Root Cause

The specification mentions that handlers should "Create operation for background processing" but this was not implemented in Ticket 3. The handlers were updated to use compose files but remained synchronous.

## Possible Resolutions

1. **Option A**: Ticket 4 cannot be completed as written since the functions to modify don't exist.

2. **Option B**: Implement the missing background operations first, then modify them to use compose files.

3. **Option C**: Accept that expose/unexpose are synchronous operations and update the documentation accordingly.

## Recommendation

Since the current synchronous implementation is already using compose files (the goal of the migration), and expose/unexpose operations are typically fast (just Caddy API calls), Option C seems most practical. The migration goal of using compose files as the source of truth has already been achieved.