-- +goose Up
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_state;
ALTER TABLE workflows ADD CONSTRAINT chk_workflows_state
  CHECK (state IN ('INITIAL', 'REVOKED', 'REJECTED', 'EXPIRED', 'WAIT_APPROVAL', 'WAIT_CONFIRMATION', 'EXECUTING', 'SUCCESSFUL', 'FAILED'));

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_action_type;
ALTER TABLE workflows ADD CONSTRAINT chk_workflows_action_type
  CHECK (action_type IN ('UPDATE_STATE', 'UPDATE_PRIMARY', 'LINK', 'UNLINK', 'SWITCH', 'DELETE'));

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_artifact_type;
ALTER TABLE workflows ADD CONSTRAINT chk_workflows_artifact_type
  CHECK (artifact_type IN ('KEY', 'KEY_CONFIGURATION', 'SYSTEM'));

ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_state;
ALTER TABLE keys ADD CONSTRAINT chk_keys_state
  CHECK (state IN ('ENABLED', 'DISABLED', 'PENDING_DELETION', 'DELETED', 'FORBIDDEN', 'UNKNOWN', 'PENDING_IMPORT', 'DETACHING', 'DETACHED'));

ALTER TABLE systems DROP CONSTRAINT IF EXISTS chk_systems_status;
ALTER TABLE systems ADD CONSTRAINT chk_systems_status
  CHECK (status IN ('CONNECTED', 'DISCONNECTED', 'FAILED', 'PROCESSING'));

-- +goose Down
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_state;
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_action_type;
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_artifact_type;
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_state;
ALTER TABLE systems DROP CONSTRAINT IF EXISTS chk_systems_status;

