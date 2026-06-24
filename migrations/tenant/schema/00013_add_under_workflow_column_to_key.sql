-- +goose Up
ALTER TABLE keys ADD COLUMN under_workflow boolean DEFAULT false;

-- +goose Down
ALTER TABLE keys DROP COLUMN IF EXISTS under_workflow;
