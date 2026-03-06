-- +goose Up
ALTER TABLE tenants DROP COLUMN region;

-- +goose Down
ALTER TABLE tenants ADD region varchar(50);
