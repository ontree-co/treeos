-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    status_level TEXT NOT NULL CHECK (status_level IN ('OK', 'WARNING', 'CRITICAL')),
    message_summary TEXT NOT NULL,
    message_details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_app_id_timestamp ON chat_messages(app_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_status_timestamp ON chat_messages(status_level, timestamp DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_chat_messages_status_timestamp;
DROP INDEX IF EXISTS idx_chat_messages_app_id_timestamp;
DROP TABLE IF EXISTS chat_messages;
-- +goose StatementEnd