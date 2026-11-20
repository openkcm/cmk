-- Extends the keys state constraint to include IMPORT_FAILED state,

-- +goose Up
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_state;
ALTER TABLE keys ADD CONSTRAINT chk_keys_state
    CHECK (state IN (
        'ENABLED', 'DISABLED', 'PENDING_DELETION', 'DELETED', 'FORBIDDEN',
        'UNKNOWN', 'PENDING_IMPORT', 'DETACHING', 'DETACHED',
        'PENDING_CREATION', 'ERROR',
        'IMPORT_FAILED'
    )) NOT VALID;

-- +goose Down
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_state;
ALTER TABLE keys ADD CONSTRAINT chk_keys_state
    CHECK (state IN (
        'ENABLED', 'DISABLED', 'PENDING_DELETION', 'DELETED', 'FORBIDDEN',
        'UNKNOWN', 'PENDING_IMPORT', 'DETACHING', 'DETACHED',
        'PENDING_CREATION', 'ERROR'
    )) NOT VALID;

