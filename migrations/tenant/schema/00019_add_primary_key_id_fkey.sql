-- Changes primary_key_id from text to uuid and adds foreign key constraint to keys table.

-- +goose Up
ALTER TABLE key_configurations ALTER COLUMN primary_key_id TYPE uuid USING primary_key_id::uuid;
ALTER TABLE key_configurations ADD CONSTRAINT fk_key_configurations_primary_key FOREIGN KEY (primary_key_id) REFERENCES keys(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE key_configurations DROP CONSTRAINT IF EXISTS fk_key_configurations_primary_key;
ALTER TABLE key_configurations ALTER COLUMN primary_key_id TYPE text;
