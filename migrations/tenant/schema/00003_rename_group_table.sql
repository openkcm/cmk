-- This migration renames the group table to groups, to follow standard naming convention

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "groups" (
	id uuid NOT NULL,
	"name" varchar(64) NOT NULL,
	description text NULL,
	"role" varchar(255) NOT NULL,
	iam_identifier varchar(128) NOT NULL,
	CONSTRAINT groups_pkey PRIMARY KEY (id),
	CONSTRAINT uni_groups_iam_identifier UNIQUE (iam_identifier),
	CONSTRAINT uni_groups_name UNIQUE (name)
);

-- Copy data from old table to new table
INSERT INTO "groups" (id, name, description, role, iam_identifier)
SELECT id, name, description, role, iam_identifier FROM "group";

-- Remove constraint fk_key_configurations_admin_group from table key_configurations
ALTER TABLE key_configurations
DROP CONSTRAINT IF EXISTS fk_key_configurations_admin_group;

-- Add foreign key constraint to key_configurations referencing the new groups table
ALTER TABLE key_configurations
ADD CONSTRAINT fk_key_configurations_admin_groups
FOREIGN KEY (admin_group_id) REFERENCES "groups" (id) ON DELETE SET NULL
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revert foreign key constraint to reference the old group table
ALTER TABLE key_configurations
DROP CONSTRAINT IF EXISTS fk_key_configurations_admin_groups;

-- Recreate previous constraint
ALTER TABLE key_configurations
ADD CONSTRAINT fk_key_configurations_admin_group
FOREIGN KEY (admin_group_id) REFERENCES "group" (id) ON DELETE SET NULL

-- Delete table created
DROP TABLE IF EXISTS "groups";
-- +goose StatementEnd
