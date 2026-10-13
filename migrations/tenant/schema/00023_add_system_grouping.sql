-- +goose Up

CREATE TABLE IF NOT EXISTS system_groups (
	id uuid NOT NULL,
	name varchar(255) NOT NULL UNIQUE,
	description varchar(255) NULL,
	suppress_warning boolean NOT NULL DEFAULT false,
	CONSTRAINT system_groups_pkey PRIMARY KEY (id)
);

ALTER TABLE systems ADD COLUMN system_group_id uuid NULL;
ALTER TABLE systems ADD CONSTRAINT fk_systems_system_group
	FOREIGN KEY (system_group_id) REFERENCES system_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_systems_system_group_id ON systems(system_group_id);

-- +goose Down

ALTER TABLE systems DROP CONSTRAINT IF EXISTS fk_systems_system_group;
DROP INDEX IF EXISTS idx_systems_system_group_id;
ALTER TABLE systems DROP COLUMN IF EXISTS system_group_id;
DROP TABLE IF EXISTS system_groups;
