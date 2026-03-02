-- +goose Up
ALTER TABLE tenants ADD name varchar(255);

-- +goose Down
ALTER TABLE tenants DROP COLUMN tenants;
