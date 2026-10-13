-- +goose Up

-- Add column with default value of 2 (system default minimum approval count)
ALTER TABLE workflows
ADD COLUMN IF NOT EXISTS minimum_approval_count INTEGER DEFAULT 2;

-- For any existing workflows, ensure they have the default value
UPDATE workflows
SET minimum_approval_count = 2
WHERE minimum_approval_count IS NULL;

-- +goose Down
ALTER TABLE workflows DROP COLUMN IF EXISTS minimum_approval_count;
