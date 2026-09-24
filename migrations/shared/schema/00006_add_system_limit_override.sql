-- +goose Up
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS system_limit_override integer;

-- +goose Down
ALTER TABLE tenants DROP COLUMN IF EXISTS system_limit_override;
