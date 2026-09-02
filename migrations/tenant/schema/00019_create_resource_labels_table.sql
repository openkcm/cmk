-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS resource_labels (
    id UUID PRIMARY KEY,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('key_configuration')),
    resource_id UUID NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Unique constraint for labels (one value per key)
    CONSTRAINT uq_resource_labels_type_id_key UNIQUE (resource_type, resource_id, key),

    -- Unique constraint for tags (multiple values allowed, but each value unique)
    CONSTRAINT uq_resource_labels_type_id_key_value UNIQUE (resource_type, resource_id, key, value)
);

CREATE INDEX idx_resource_labels_resource ON resource_labels(resource_type, resource_id);
CREATE INDEX idx_resource_labels_key ON resource_labels(key);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS resource_labels;
-- +goose StatementEnd
