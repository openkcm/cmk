-- +goose Up
-- Drop existing key_versions table and constraints
DROP TABLE IF EXISTS key_versions CASCADE;

-- Recreate with new schema
CREATE TABLE IF NOT EXISTS key_versions (
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    id uuid NOT NULL,
    native_id varchar(255) NOT NULL,
    key_id uuid NOT NULL,
    rotated_at timestamptz NOT NULL,
    CONSTRAINT key_versions_pkey PRIMARY KEY (id),
    CONSTRAINT fk_keys_key_versions FOREIGN KEY (key_id) REFERENCES "keys"(id) ON DELETE CASCADE
);

-- Index on key_id for efficient lookups of all versions for a key
CREATE INDEX IF NOT EXISTS idx_key_versions_key_id ON key_versions USING btree (key_id);

-- Unique constraint on (key_id, native_id) to prevent duplicate versions from same keystore
CREATE UNIQUE INDEX IF NOT EXISTS idx_key_versions_key_native ON key_versions USING btree (key_id, native_id);

-- Index on rotated_at for efficient sorting (finding latest version)
-- Latest (most recent) RotatedAt = current version
CREATE INDEX IF NOT EXISTS idx_key_versions_rotated_at ON key_versions USING btree (key_id, rotated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS key_versions CASCADE;

CREATE TABLE IF NOT EXISTS key_versions (
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    external_id varchar(255) NOT NULL,
    native_id varchar(255) NULL,
    key_id uuid NOT NULL,
    "version" int8 DEFAULT 0 NOT NULL,
    is_primary bool DEFAULT false NOT NULL,
    CONSTRAINT key_versions_pkey PRIMARY KEY (external_id),
    CONSTRAINT fk_keys_key_versions FOREIGN KEY (key_id) REFERENCES "keys"(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS key_version ON key_versions USING btree (key_id, version);

