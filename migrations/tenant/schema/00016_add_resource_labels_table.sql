-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS resource_labels (
    id uuid NOT NULL,
    resource_type varchar(50) NOT NULL,
    resource_id uuid NOT NULL,
    key varchar(255) NOT NULL,
    value varchar(255) NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT resource_labels_pkey PRIMARY KEY (id)
);

-- Enable filtering by resource (resource_type + resource_id combination)
CREATE INDEX idx_resource_labels_resource_type_id
    ON resource_labels(resource_type, resource_id);

-- Enable filtering by label/tag key
CREATE INDEX idx_resource_labels_key
    ON resource_labels(key);

-- Enforce uniqueness: one value per (resource_type, resource_id, key) for labels
-- Exception: system.tag key can have multiple values (multiple tags per resource)
-- This prevents duplicate keys and matches label semantics (update by key, delete by key)
CREATE UNIQUE INDEX idx_resource_labels_unique_key
    ON resource_labels(resource_type, resource_id, key)
    WHERE key != 'system.tag';

-- For system tags, allow multiple values but prevent exact duplicates
CREATE UNIQUE INDEX idx_resource_labels_unique_tag
    ON resource_labels(resource_type, resource_id, key, value)
    WHERE key = 'system.tag';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resource_labels_unique_tag;
DROP INDEX IF EXISTS idx_resource_labels_unique_key;
DROP INDEX IF EXISTS idx_resource_labels_key;
DROP INDEX IF EXISTS idx_resource_labels_resource_type_id;
DROP TABLE IF EXISTS resource_labels;
-- +goose StatementEnd
