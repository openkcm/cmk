-- +goose Up
ALTER TABLE systems ADD COLUMN under_workflow boolean DEFAULT false;

-- +goose Down
ALTER TABLE systems DROP COLUMN IF EXISTS under_workflow;
