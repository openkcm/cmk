-- +goose Up
ALTER TABLE events ALTER COLUMN error_message type TEXT;

-- +goose Down
ALTER TABLE events ALTER COLUMN error_message type VARCHAR(256);
