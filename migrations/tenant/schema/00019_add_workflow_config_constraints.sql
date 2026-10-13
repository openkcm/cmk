-- Adds CHECK constraints on workflow configuration values.
-- Any existing out-of-bounds rows are clamped by the accompanying data
-- migration (migrations/tenant/data/00002_clamp_workflow_config_bounds.go)
-- which runs in parallel.
--
-- Hard limits (matching constants/workflow.go):
--   minimumApprovals : 2 – 5
--   retentionPeriodDays : 7 – 30  (must be >= maxExpiryPeriodDays)
--   maxExpiryPeriodDays : 1 – 7

-- +goose Up

-- Add CHECK constraints (only enforced on the WORKFLOW_CONFIG row).
ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_minimum_approvals
    CHECK (
        key != 'WORKFLOW_CONFIG'
        OR (value->>'minimumApprovals')::int BETWEEN 2 AND 5
    ) NOT VALID;

ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_retention_period_days
    CHECK (
        key != 'WORKFLOW_CONFIG'
        OR (value->>'retentionPeriodDays')::int BETWEEN 7 AND 30
    ) NOT VALID;

ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_max_expiry_period_days
    CHECK (
        key != 'WORKFLOW_CONFIG'
        OR (value->>'maxExpiryPeriodDays')::int BETWEEN 1 AND 7
    ) NOT VALID;

-- +goose Down
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_minimum_approvals;
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_retention_period_days;
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_max_expiry_period_days;
