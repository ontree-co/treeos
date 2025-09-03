-- +goose Up
-- +goose StatementBegin
-- First, we need to drop the constraint on status_level to allow user messages
-- SQLite doesn't support ALTER TABLE DROP CONSTRAINT, so we need to recreate the table

-- Create a new table without the constraint
CREATE TABLE IF NOT EXISTS chat_messages_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    status_level TEXT NOT NULL,
    message_summary TEXT NOT NULL,
    message_details TEXT,
    message_type TEXT DEFAULT 'agent', -- 'agent' or 'user'
    sender_name TEXT DEFAULT 'System Agent', -- Name of sender (e.g., 'System Agent' or 'User')
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Copy existing data from old table
INSERT INTO chat_messages_new (id, app_id, timestamp, status_level, message_summary, message_details, created_at)
SELECT id, app_id, timestamp, status_level, message_summary, message_details, created_at
FROM chat_messages;

-- Drop old table
DROP TABLE chat_messages;

-- Rename new table
ALTER TABLE chat_messages_new RENAME TO chat_messages;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_chat_messages_app_id_timestamp ON chat_messages(app_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_status_timestamp ON chat_messages(status_level, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_type ON chat_messages(message_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert to original schema with constraint
CREATE TABLE IF NOT EXISTS chat_messages_original (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    status_level TEXT NOT NULL CHECK (status_level IN ('OK', 'WARNING', 'CRITICAL')),
    message_summary TEXT NOT NULL,
    message_details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Copy data back, filtering only agent messages with valid status levels
INSERT INTO chat_messages_original (id, app_id, timestamp, status_level, message_summary, message_details, created_at)
SELECT id, app_id, timestamp, status_level, message_summary, message_details, created_at
FROM chat_messages
WHERE message_type = 'agent' AND status_level IN ('OK', 'WARNING', 'CRITICAL');

-- Drop the modified table
DROP TABLE chat_messages;

-- Rename back
ALTER TABLE chat_messages_original RENAME TO chat_messages;

-- Recreate original indexes
CREATE INDEX IF NOT EXISTS idx_chat_messages_app_id_timestamp ON chat_messages(app_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_status_timestamp ON chat_messages(status_level, timestamp DESC);
-- +goose StatementEnd