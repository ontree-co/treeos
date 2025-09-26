-- Migration: Rename docker_* tables to container_*
-- This migration renames all Docker-related tables to use the more generic "container" terminology
-- as we've migrated from Docker to Podman

-- Rename docker_operations to container_operations
ALTER TABLE docker_operations RENAME TO container_operations;

-- Rename docker_operation_logs to container_operation_logs
ALTER TABLE docker_operation_logs RENAME TO container_operation_logs;

-- Update indexes to match new table names
DROP INDEX IF EXISTS idx_docker_operations_status_created;
DROP INDEX IF EXISTS idx_docker_operations_app_created;
DROP INDEX IF EXISTS idx_docker_operation_logs_operation_timestamp;

CREATE INDEX IF NOT EXISTS idx_container_operations_status_created ON container_operations(status, created_at);
CREATE INDEX IF NOT EXISTS idx_container_operations_app_created ON container_operations(app_name, created_at);
CREATE INDEX IF NOT EXISTS idx_container_operation_logs_operation_timestamp ON container_operation_logs(operation_id, timestamp);