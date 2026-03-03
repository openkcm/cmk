-- This migration renames the group table to groups, to follow standard naming convention
-- It also creates a trigger to sync writes and deletes on both tables

-- +goose Up
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

-- Create trigger to sync group and groups tables 
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_group_to_groups()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO groups (id, name, description, role, iam_identifier)
        VALUES (NEW.id, NEW.name, NEW.description, NEW.role, NEW.iam_identifier)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            description = EXCLUDED.description,
            role = EXCLUDED.role,
            iam_identifier = EXCLUDED.iam_identifier;
        RETURN NEW;
        
    ELSIF TG_OP = 'UPDATE' THEN
        UPDATE groups SET
            name = NEW.name,
            description = NEW.description,
            role = NEW.role,
            iam_identifier = NEW.iam_identifier
        WHERE id = NEW.id;
        RETURN NEW;
        
    ELSIF TG_OP = 'DELETE' THEN
        DELETE FROM groups WHERE id = OLD.id;
        RETURN OLD;
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_sync_group_to_groups
AFTER INSERT OR UPDATE OR DELETE ON "group"
FOR EACH ROW
EXECUTE FUNCTION sync_group_to_groups();

-- +goose Down
DROP TRIGGER IF EXISTS trigger_sync_group_to_groups ON "group";
DROP FUNCTION IF EXISTS sync_group_to_groups();
DROP TABLE IF EXISTS "groups";
