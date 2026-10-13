-- Extends the keys state constraint to include PENDING_CREATION and ERROR states,
-- and adds error_detail JSONB column for structured failure information.

-- +goose Up
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_state;
ALTER TABLE keys ADD CONSTRAINT chk_keys_state
    CHECK (state IN (
        'ENABLED', 'DISABLED', 'PENDING_DELETION', 'DELETED', 'FORBIDDEN',
        'UNKNOWN', 'PENDING_IMPORT', 'DETACHING', 'DETACHED',
        'PENDING_CREATION', 'ERROR'
    )) NOT VALID;

ALTER TABLE keys ADD COLUMN IF NOT EXISTS error_detail JSONB;

-- +goose Down
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_state;
ALTER TABLE keys ADD CONSTRAINT chk_keys_state
    CHECK (state IN (
        'ENABLED', 'DISABLED', 'PENDING_DELETION', 'DELETED', 'FORBIDDEN',
        'UNKNOWN', 'PENDING_IMPORT', 'DETACHING', 'DETACHED'
    )) NOT VALID;

ALTER TABLE keys DROP COLUMN IF EXISTS error_detail;
