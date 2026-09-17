-- Improve performance of pending_approvals aggregation in key configuration queries
-- +goose Up

-- Helps filter workflows by non-terminal state in the pending approvals JOIN
CREATE INDEX IF NOT EXISTS idx_workflows_state ON workflows(state);

-- Covering index for the workflow_key_configurations lookup by key_configuration_id,
-- including workflow_id to avoid heap fetches during the JOIN
CREATE INDEX IF NOT EXISTS idx_wkc_kc_wf ON workflow_key_configurations(key_configuration_id, workflow_id);

-- +goose Down
DROP INDEX IF EXISTS idx_wkc_kc_wf;
DROP INDEX IF EXISTS idx_workflows_state;
