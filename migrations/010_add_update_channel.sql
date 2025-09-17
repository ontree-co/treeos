-- +goose Up
-- +goose StatementBegin
-- Add update channel field to system_setup table
ALTER TABLE system_setup ADD COLUMN update_channel TEXT DEFAULT 'beta';

-- Create update history table for tracking update attempts
CREATE TABLE IF NOT EXISTS update_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version TEXT NOT NULL,
    channel TEXT NOT NULL,
    status TEXT NOT NULL, -- 'success', 'failed', 'rolled_back'
    error_message TEXT,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add index for efficient queries
CREATE INDEX idx_update_history_started_at ON update_history(started_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove update channel field
ALTER TABLE system_setup DROP COLUMN update_channel;

-- Drop update history table
DROP TABLE IF EXISTS update_history;
-- +goose StatementEnd