-- Rewrites legacy DEFAULT_KEYSTORE blobs from flat shape to nested
-- roleManagementConfig shape, so 00002 backfill sees the canonical layout.
-- JSON-key literals are part of the structure and not extracted.
-- +goose Up

UPDATE tenant_configs
SET value = jsonb_build_object(
        'roleManagementConfig', jsonb_build_object( -- NOSONAR
            'localityId', value::jsonb -> 'localityId', -- NOSONAR
            'commonName', value::jsonb -> 'commonName', -- NOSONAR
            'accessData', value::jsonb -> 'managementAccessData'
        ),
        'supportedRegions', value::jsonb -> 'supportedRegions' -- NOSONAR
    )::jsonb
WHERE "key" = 'DEFAULT_KEYSTORE'
  AND value::jsonb ? 'localityId'
  AND NOT (value::jsonb ? 'roleManagementConfig');

-- +goose Down

UPDATE tenant_configs
SET value = jsonb_build_object(
        'localityId',           value::jsonb -> 'roleManagementConfig' -> 'localityId',
        'commonName',           value::jsonb -> 'roleManagementConfig' -> 'commonName',
        'managementAccessData', value::jsonb -> 'roleManagementConfig' -> 'accessData',
        'supportedRegions',     value::jsonb -> 'supportedRegions'
    )::jsonb
WHERE "key" = 'DEFAULT_KEYSTORE'
  AND value::jsonb ? 'roleManagementConfig';
