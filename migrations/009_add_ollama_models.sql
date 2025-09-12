-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ollama_models (
    name TEXT PRIMARY KEY,                    -- e.g., "llama3:8b"
    display_name TEXT NOT NULL,               -- e.g., "Llama 3 8B"
    size_estimate TEXT,                       -- e.g., "4.5GB"
    description TEXT,
    category TEXT,                            -- e.g., "chat", "code", "vision"
    status TEXT DEFAULT 'not_downloaded',     -- not_downloaded, queued, downloading, completed, failed
    progress INTEGER DEFAULT 0,               -- 0-100
    last_error TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS ollama_download_jobs (
    id TEXT PRIMARY KEY,
    model_name TEXT NOT NULL,
    status TEXT DEFAULT 'queued',             -- queued, processing, completed, failed
    started_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (model_name) REFERENCES ollama_models(name)
);

CREATE INDEX IF NOT EXISTS idx_ollama_models_status ON ollama_models(status);
CREATE INDEX IF NOT EXISTS idx_download_jobs_status ON ollama_download_jobs(status, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_download_jobs_status;
DROP INDEX IF EXISTS idx_ollama_models_status;
DROP TABLE IF EXISTS ollama_download_jobs;
DROP TABLE IF EXISTS ollama_models;
-- +goose StatementEnd