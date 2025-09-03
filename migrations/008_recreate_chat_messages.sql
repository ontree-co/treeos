-- +goose Up
-- +goose StatementBegin
-- Drop the old table completely
DROP TABLE IF EXISTS chat_messages;

-- Create new clean schema with proper sender identity
CREATE TABLE chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    -- Core message content
    message TEXT NOT NULL,
    
    -- Sender identity
    sender_type TEXT NOT NULL CHECK (sender_type IN ('user', 'agent', 'system')),
    sender_name TEXT NOT NULL,
    
    -- Agent-specific metadata (NULL for non-agent messages)
    agent_model TEXT,
    agent_provider TEXT,
    status_level TEXT CHECK (status_level IN ('info', 'warning', 'error', 'critical') OR status_level IS NULL),
    
    -- Optional details for any message type
    details TEXT,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_chat_messages_app_timestamp ON chat_messages(app_id, timestamp DESC);
CREATE INDEX idx_chat_messages_sender_type ON chat_messages(sender_type, timestamp DESC);
CREATE INDEX idx_chat_messages_app_sender ON chat_messages(app_id, sender_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chat_messages;
-- +goose StatementEnd