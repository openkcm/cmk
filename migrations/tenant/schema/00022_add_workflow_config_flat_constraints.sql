-- Re-expresses the workflow-config hard limits (schema 00019, dropped in 00021
-- with the jsonb column) as CHECK constraints on the flat "workflow" rows.
--
-- Hard limits (matching constants/workflow.go):
--   minimum_approvals    : 2 – 5
--   retention_period_days : 7 – 30
--   max_expiry_period_days : 1 – 7

-- +goose Up

-- CASE guards the cast: OR is not guaranteed to short-circuit, so a bare guard
-- can still cast (and throw on) non-numeric rows. The regex rejects malformed
-- workflow values cleanly.
ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_minimum_approvals
    CHECK (
        CASE WHEN "type" = 'workflow' AND "key" = 'minimum_approvals'
             THEN value ~ '^\d+$' AND value::int BETWEEN 2 AND 5
             ELSE true
        END
    ) NOT VALID;

ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_retention_period_days
    CHECK (
        CASE WHEN "type" = 'workflow' AND "key" = 'retention_period_days'
             THEN value ~ '^\d+$' AND value::int BETWEEN 7 AND 30
             ELSE true
        END
    ) NOT VALID;

ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_max_expiry_period_days
    CHECK (
        CASE WHEN "type" = 'workflow' AND "key" = 'max_expiry_period_days'
             THEN value ~ '^\d+$' AND value::int BETWEEN 1 AND 7
             ELSE true
        END
    ) NOT VALID;

-- +goose Down
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_minimum_approvals;
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_retention_period_days;
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_max_expiry_period_days;
