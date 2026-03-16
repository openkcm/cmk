-- +goose Up
CREATE SCHEMA IF NOT EXISTS "orbital";

-- +goose Down
DROP SCHEMA IF EXISTS "orbital";
