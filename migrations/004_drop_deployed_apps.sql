-- +goose Up
-- +goose StatementBegin
-- Drop indexes first
DROP INDEX IF EXISTS idx_deployed_apps_exposed;
DROP INDEX IF EXISTS idx_deployed_apps_name;

-- Drop the deployed_apps table
DROP TABLE IF EXISTS deployed_apps;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Recreate the deployed_apps table and indexes
CREATE TABLE IF NOT EXISTS deployed_apps (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    docker_compose TEXT NOT NULL,
    subdomain TEXT,
    host_port INTEGER,
    is_exposed INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_deployed_apps_name ON deployed_apps(name);
CREATE INDEX IF NOT EXISTS idx_deployed_apps_exposed ON deployed_apps(is_exposed);
-- +goose StatementEnd