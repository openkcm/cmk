-- This migration is not following the defined behavior for migrations
-- as it's altering a table in a single step
-- This was decided as this table was empty accross the environments 
-- as this feature was not completed at the time of writing this script
-- and no productive environment exists
--
-- +goose Up
ALTER TABLE keystore_configurations RENAME COLUMN value TO config;
ALTER TABLE keystore_configurations RENAME TO keystore_pool;

-- +goose Down
ALTER TABLE keystore_pool RENAME TO keystore_configurations;
ALTER TABLE keystore_configurations RENAME COLUMN config TO value;
