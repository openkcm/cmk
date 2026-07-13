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

-- Enable combined filtering and enforce uniqueness per resource
-- Prevents duplicate (resource_type, resource_id, key, value) combinations
CREATE UNIQUE INDEX idx_resource_labels_unique
    ON resource_labels(resource_type, resource_id, key, value);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resource_labels_unique;
DROP INDEX IF EXISTS idx_resource_labels_key;
DROP INDEX IF EXISTS idx_resource_labels_resource_type_id;
DROP TABLE IF EXISTS resource_labels;
-- +goose StatementEnd
