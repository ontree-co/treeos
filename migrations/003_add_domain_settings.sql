-- +goose Up
-- +goose StatementBegin
ALTER TABLE system_setup ADD COLUMN public_base_domain TEXT;
ALTER TABLE system_setup ADD COLUMN tailscale_base_domain TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE system_setup DROP COLUMN public_base_domain;
ALTER TABLE system_setup DROP COLUMN tailscale_base_domain;
-- +goose StatementEnd