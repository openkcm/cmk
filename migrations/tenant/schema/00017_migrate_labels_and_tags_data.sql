-- +goose Up
-- +goose StatementBegin

-- Migrate existing key_labels to resource_labels
-- Maps: key_labels -> resource_labels with resource_type='KEY'
-- Note: With the new unique constraint on (resource_type, resource_id, key),
-- if there are duplicate keys, only the most recently updated one is kept
INSERT INTO resource_labels (id, resource_type, resource_id, key, value, created_at, updated_at)
SELECT DISTINCT ON (resource_id, key)
    id,
    'KEY' AS resource_type,
    resource_id,
    key,
    COALESCE(value, '') AS value,  -- Ensure value is not null
    created_at,
    updated_at
FROM key_labels
ORDER BY resource_id, key, updated_at DESC  -- Keep most recent if duplicates exist
ON CONFLICT (resource_type, resource_id, key) WHERE key != 'system.tag' DO NOTHING;

-- Migrate existing key_configuration tags to resource_labels
-- Maps: tags table (JSONB values) -> resource_labels
-- with resource_type='KEY_CONFIG' and key='system.tag'
-- The tags table was created in migration 00002, replacing the old keyconfigurations_tags structure
DO $$
BEGIN
    -- Check if tags table exists (it should, but check for safety)
    IF to_regclass('tags') IS NOT NULL THEN
        INSERT INTO resource_labels (id, resource_type, resource_id, key, value, created_at, updated_at)
        SELECT
            gen_random_uuid() AS id,
            'KEY_CONFIG' AS resource_type,
            t.id AS resource_id,  -- tags.id is the key_configuration_id
            'system.tag' AS key,
            tag_value::text AS value,  -- Extract each tag from JSONB array
            CURRENT_TIMESTAMP AS created_at,
            CURRENT_TIMESTAMP AS updated_at
        FROM tags t,
        LATERAL jsonb_array_elements_text(t.values) AS tag_value
        WHERE t.values IS NOT NULL AND jsonb_array_length(t.values) > 0
        ON CONFLICT (resource_type, resource_id, key, value) WHERE key = 'system.tag' DO NOTHING;
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
