-- +goose Up
ALTER TABLE key_versions ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN';

-- +goose Down
ALTER TABLE key_versions DROP COLUMN status;
