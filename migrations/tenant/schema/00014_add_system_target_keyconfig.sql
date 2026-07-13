-- +goose Up
ALTER TABLE systems ADD COLUMN target_key_configuration_id uuid DEFAULT NULL;

-- +goose Down
ALTER TABLE systems DROP COLUMN IF EXISTS target_key_configuration_id;
