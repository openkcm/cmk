-- +goose Up
-- Add status field to keystore_pool table to track pending/active/failed/orphaned keystores
ALTER TABLE keystore_pool
    ADD COLUMN status varchar(50) NOT NULL DEFAULT 'ACTIVE';

-- Update existing rows to have status = 'ACTIVE' (for backwards compatibility)
UPDATE keystore_pool SET status = 'ACTIVE' WHERE status IS NULL;

-- Create index for efficient querying by status
CREATE INDEX idx_keystore_pool_status ON keystore_pool(status);

-- Create partial unique index on config field (only for ACTIVE keystores)
-- This allows PENDING keystores to have placeholder/empty configs
-- First, we need to drop the existing constraint if it exists as a constraint not an index
ALTER TABLE keystore_pool DROP CONSTRAINT IF EXISTS uni_public_keystore_configurations_value;

-- Then drop the index if it exists
DROP INDEX IF EXISTS uni_public_keystore_configurations_value;

-- Create the new partial unique index
CREATE UNIQUE INDEX uni_keystore_pool_config_active
    ON keystore_pool(config)
    WHERE status = 'ACTIVE';

-- +goose Down
-- Restore original unique constraint on config
DROP INDEX IF EXISTS uni_keystore_pool_config_active;
CREATE UNIQUE INDEX uni_public_keystore_configurations_value ON keystore_pool(config);

-- Remove status index
DROP INDEX IF EXISTS idx_keystore_pool_status;

-- Remove status column
ALTER TABLE keystore_pool DROP COLUMN status;
