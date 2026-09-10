-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS resource_labels (
    id UUID PRIMARY KEY,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('key_configuration','key')),
    resource_id UUID NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

-- Partial unique index for labels: one value per (resource_type, resource_id, key) when key != 'system.tag'
CREATE UNIQUE INDEX uq_resource_labels_type_id_key
    ON resource_labels(resource_type, resource_id, key)
    WHERE key != 'system.tag';

-- Partial unique index for tags: each (resource_type, resource_id, key, value) must be unique when key = 'system.tag'
CREATE UNIQUE INDEX uq_resource_labels_type_id_key_value
    ON resource_labels(resource_type, resource_id, key, value)
    WHERE key = 'system.tag';

CREATE INDEX idx_resource_labels_resource ON resource_labels(resource_type, resource_id);
CREATE INDEX idx_resource_labels_key ON resource_labels(key);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS resource_labels;
-- +goose StatementEnd
