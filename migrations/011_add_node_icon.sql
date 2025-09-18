-- +goose Up
-- +goose StatementBegin
ALTER TABLE system_setup ADD COLUMN node_icon TEXT DEFAULT 'tree1.png';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE system_setup DROP COLUMN node_icon;
-- +goose StatementEnd