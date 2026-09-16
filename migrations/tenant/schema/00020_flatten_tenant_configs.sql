-- This migration adds the flat-row shape to tenant_configs additively: a type
-- discriminator, a text value column, and a unique (key, type) index. The legacy
-- value (jsonb) column and (key) primary key stay intact and are removed later.
-- +goose Up

ALTER TABLE tenant_configs ADD COLUMN IF NOT EXISTS "type" varchar(255) NOT NULL DEFAULT '';

ALTER TABLE tenant_configs ADD COLUMN IF NOT EXISTS value_text text;

-- Flat rows populate value_text instead of the legacy value column.
ALTER TABLE tenant_configs ALTER COLUMN value DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_configs_key_type ON tenant_configs ("key", "type");

CREATE INDEX IF NOT EXISTS idx_tenant_configs_type ON tenant_configs ("type");

-- +goose Down

DROP INDEX IF EXISTS idx_tenant_configs_type;
DROP INDEX IF EXISTS idx_tenant_configs_key_type;

-- Remove flat rows (they only populate value_text) so the legacy value column
-- can be restored to NOT NULL.
DELETE FROM tenant_configs WHERE length("type") > 0;

ALTER TABLE tenant_configs DROP COLUMN IF EXISTS value_text;
ALTER TABLE tenant_configs DROP COLUMN IF EXISTS "type";

ALTER TABLE tenant_configs ALTER COLUMN value SET NOT NULL;
