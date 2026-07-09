-- This migration completes the flatten by settling tenant_configs into the
-- flat-only shape. It backfills any legacy blob still present into value_text,
-- drops the legacy value column, renames value_text to value, and promotes the
-- primary key to (key, type). Runs only after all readers use flat rows.
-- +goose Up

-- Capture any legacy blob a prior release left un-flattened before dropping it.
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT target_key, value::jsonb ->> source_key, 'workflow'
FROM tenant_configs,
     LATERAL (VALUES
        ('Enabled',                 'enabled'),
        ('MinimumApprovals',        'minimum_approvals'),
        ('RetentionPeriodDays',     'retention_period_days'),
        ('DefaultExpiryPeriodDays', 'default_expiry_period_days'),
        ('MaxExpiryPeriodDays',     'max_expiry_period_days')
     ) AS keys(source_key, target_key)
WHERE "key" = 'WORKFLOW_CONFIG' AND length("type") = 0
  AND value::jsonb ? source_key
ON CONFLICT ("key", "type") DO NOTHING;

INSERT INTO tenant_configs ("key", value_text, "type")
SELECT target_key,
       CASE source_key
           WHEN 'accessData' THEN (rmc -> source_key)::text
           ELSE rmc ->> source_key
       END,
       'default_keystore'
FROM tenant_configs,
     LATERAL (SELECT value::jsonb -> 'roleManagementConfig' AS rmc) AS rm,
     LATERAL (VALUES
        ('localityId', 'locality_id'),
        ('commonName', 'common_name'),
        ('accessData', 'management_access_data')
     ) AS keys(source_key, target_key)
WHERE "key" = 'DEFAULT_KEYSTORE' AND length("type") = 0
  AND rmc IS NOT NULL AND rmc ? source_key
ON CONFLICT ("key", "type") DO NOTHING;

INSERT INTO tenant_configs ("key", value_text, "type")
SELECT target_key, (value::jsonb -> source_key)::text, 'default_keystore'
FROM tenant_configs,
     LATERAL (VALUES
        ('keyManagementConfig', 'key_management_config'),
        ('cryptoAccessData',    'crypto_access_data'),
        ('supportedRegions',    'supported_regions')
     ) AS keys(source_key, target_key)
WHERE "key" = 'DEFAULT_KEYSTORE' AND length("type") = 0
  AND value::jsonb ? source_key
ON CONFLICT ("key", "type") DO NOTHING;

-- Drop the now-redundant legacy blob rows.
DELETE FROM tenant_configs
WHERE length("type") = 0
  AND "key" IN ('WORKFLOW_CONFIG', 'DEFAULT_KEYSTORE');

-- Settle the column layout: value_text becomes the sole value column.
ALTER TABLE tenant_configs DROP COLUMN IF EXISTS value;
ALTER TABLE tenant_configs RENAME COLUMN value_text TO value;
ALTER TABLE tenant_configs ALTER COLUMN value SET NOT NULL;

-- Promote (key, type) to the primary key; the interim unique index is redundant.
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS tenant_configs_pkey;
ALTER TABLE tenant_configs ADD CONSTRAINT tenant_configs_pkey PRIMARY KEY ("key", "type");
DROP INDEX IF EXISTS idx_tenant_configs_key_type;

-- +goose Down

-- Restore the post-expand shape: a nullable legacy value column, the value_text
-- column, the (key) primary key, and the interim (key, type) unique index.
ALTER TABLE tenant_configs RENAME COLUMN value TO value_text;
ALTER TABLE tenant_configs ADD COLUMN IF NOT EXISTS value jsonb;

ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS tenant_configs_pkey;
ALTER TABLE tenant_configs ADD CONSTRAINT tenant_configs_pkey PRIMARY KEY ("key");
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_configs_key_type ON tenant_configs ("key", "type");
