-- This migration is not following the defined behavior for migrations
-- as it's creating a new table and destroying the previous one in the same migration
-- breaking the two-step approach. 
-- This was decided as the tags functionally was previously broken creating corrupted data.
-- At the time of creation of this migration, there were no productive environments
-- meaning the loss of this corrupted data is acceptable
--
-- +goose Up

CREATE TABLE tags (
	id uuid NOT NULL,
	"values" jsonb NULL,
	CONSTRAINT tags_pkey PRIMARY KEY (id)
);

DROP TABLE IF EXISTS keyconfigurations_tags;
DROP TABLE IF EXISTS key_configuration_tags;

-- +goose Down
DROP TABLE IF EXISTS tags;
