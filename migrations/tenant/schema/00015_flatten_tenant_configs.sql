-- This migration flattens tenant_configs from a single JSON blob per key into
-- (key, value, type) flat rows so individual config entries can be filtered by
-- type/key. Data migration 00002_flatten_tenant_configs.go backfills flat rows
-- from the legacy blobs; the legacy blobs are dropped in a future release.
--
-- Down drops flat rows. Since post-flatten writes target flat rows only, Down
-- discards any changes made after the migration. Use only before traffic;
-- restore from backup for operational rollback after production writes.
-- +goose Up

ALTER TABLE tenant_configs ADD COLUMN IF NOT EXISTS "type" varchar(255) NOT NULL DEFAULT '';

ALTER TABLE tenant_configs ALTER COLUMN value TYPE text USING value::text;

ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS tenant_configs_pkey;
ALTER TABLE tenant_configs ADD CONSTRAINT tenant_configs_pkey PRIMARY KEY ("key", "type");

CREATE INDEX IF NOT EXISTS idx_tenant_configs_type ON tenant_configs ("type");

-- +goose Down

DROP INDEX IF EXISTS idx_tenant_configs_type;

-- Flat rows hold non-JSON text and would fail the jsonb cast below.
DELETE FROM tenant_configs WHERE length("type") > 0;

ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS tenant_configs_pkey;
ALTER TABLE tenant_configs DROP COLUMN IF EXISTS "type";
ALTER TABLE tenant_configs ALTER COLUMN value TYPE jsonb USING value::jsonb;
ALTER TABLE tenant_configs ADD CONSTRAINT tenant_configs_pkey PRIMARY KEY ("key");
