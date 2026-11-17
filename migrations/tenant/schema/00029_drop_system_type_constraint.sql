-- System types are validated using configurableContext.system.type.enum.
-- +goose Up
ALTER TABLE systems DROP CONSTRAINT IF EXISTS chk_systems_type;

-- +goose Down
ALTER TABLE systems ADD CONSTRAINT chk_systems_type
    CHECK (type IN ('SYSTEM', 'SUBACCOUNT')) NOT VALID;
