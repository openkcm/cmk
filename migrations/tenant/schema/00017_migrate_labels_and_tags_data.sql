-- +goose Up
-- +goose StatementBegin

-- Migrate existing key_labels to resource_labels
-- Maps: key_labels -> resource_labels with resource_type='KEY'
INSERT INTO resource_labels (id, resource_type, resource_id, key, value, created_at, updated_at)
SELECT
    id,
    'KEY' AS resource_type,
    resource_id,
    key,
    COALESCE(value, '') AS value,  -- Ensure value is not null
    created_at,
    updated_at
FROM key_labels
ON CONFLICT (resource_type, resource_id, key, value) DO NOTHING;

-- Migrate existing key_configuration_tags to resource_labels
-- Maps: key_configuration_tags + keyconfigurations_tags -> resource_labels
-- with resource_type='KEY_CONFIG' and key='system.tag'
DO $$
BEGIN
    -- These legacy tables may not exist on fresh installs (they are dropped in migration 00002).
    IF to_regclass('public.keyconfigurations_tags') IS NOT NULL
       AND to_regclass('public.key_configuration_tags') IS NOT NULL THEN
        INSERT INTO resource_labels (id, resource_type, resource_id, key, value, created_at, updated_at)
        SELECT
            gen_random_uuid() AS id,  -- Generate new UUIDs for tag entries
            'KEY_CONFIG' AS resource_type,
            kt.key_configuration_id AS resource_id,
            'system.tag' AS key,
            t.value AS value,
            CURRENT_TIMESTAMP AS created_at,
            CURRENT_TIMESTAMP AS updated_at
        FROM keyconfigurations_tags kt
        JOIN key_configuration_tags t ON kt.key_configuration_tag_id = t.id
        ON CONFLICT (resource_type, resource_id, key, value) DO NOTHING;
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove migrated key labels (based on resource_type='KEY')
DELETE FROM resource_labels
WHERE resource_type = 'KEY'
AND id IN (SELECT id FROM key_labels);

-- Remove migrated key configuration tags (based on resource_type='KEY_CONFIG' and key='system.tag')
-- Note: Since we generated new UUIDs during migration, we can't match by ID
-- Instead, we delete all KEY_CONFIG entries with system.tag key
DELETE FROM resource_labels
WHERE resource_type = 'KEY_CONFIG'
AND key = 'system.tag';

-- +goose StatementEnd
