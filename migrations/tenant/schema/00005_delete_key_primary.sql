-- This can be safely removed as it only mirrors the keyconfig primary key ID
-- +goose Up
ALTER TABLE keys DROP is_primary;

-- +goose Down
ALTER TABLE keys ADD is_primary bool NULL;
