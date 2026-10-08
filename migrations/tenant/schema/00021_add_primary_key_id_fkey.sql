-- Changes primary_key_id from text to uuid and adds foreign key constraint to keys table.

-- +goose Up
ALTER TABLE key_configurations ALTER COLUMN primary_key_id TYPE uuid USING primary_key_id::uuid;

-- Need to run update before create the fkey. This is done here as it's needed before applying the fkey 
-- It's also fine to run it here as of the time of creation this does not result in high processing time
-- However this is an exception and generally should be a data migration
UPDATE key_configurations
SET primary_key_id = NULL
WHERE primary_key_id IS NOT NULL
  AND primary_key_id NOT IN (SELECT id FROM keys);

ALTER TABLE key_configurations ADD CONSTRAINT fk_key_configurations_primary_key FOREIGN KEY (primary_key_id) REFERENCES keys(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE key_configurations DROP CONSTRAINT IF EXISTS fk_key_configurations_primary_key;
ALTER TABLE key_configurations ALTER COLUMN primary_key_id TYPE text;
