-- +goose Up
-- Update system_setup table for new Tailscale configuration

-- Remove old tailscale_base_domain column and add new ones
ALTER TABLE system_setup DROP COLUMN IF EXISTS tailscale_base_domain;
ALTER TABLE system_setup ADD COLUMN IF NOT EXISTS tailscale_auth_key TEXT;
ALTER TABLE system_setup ADD COLUMN IF NOT EXISTS tailscale_tags TEXT DEFAULT 'tag:ontree-apps';

-- +goose Down
-- Revert changes
ALTER TABLE system_setup DROP COLUMN IF EXISTS tailscale_auth_key;
ALTER TABLE system_setup DROP COLUMN IF EXISTS tailscale_tags;
ALTER TABLE system_setup ADD COLUMN IF NOT EXISTS tailscale_base_domain TEXT;