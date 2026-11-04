-- Renames flat workflow config keys to match the updated WorkflowConfig model
-- fields introduced in internal/model/tenantconfigs.go.
--
-- Key renames:
--   minimum_approvals        → required_approvals (also copied to min_allowed_approvals)
--   max_approvals            → max_allowed_approvals
--   min_retention_period_days → min_allowed_retention_period_days
--   max_retention_period_days → max_allowed_retention_period_days
--   default_expiry_period_days → expiry_period_days
--   max_expiry_period_days   → max_allowed_expiry_period_days
--
-- Also replaces CHECK constraints from 00025 that referenced the old key names.

-- +goose Up

-- Drop old constraints (referenced old key names)
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_minimum_approvals;
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_max_expiry_period_days;

-- Rename keys (data migration handled by data migration 00007)

-- Add new constraints for renamed keys
ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_required_approvals
    CHECK (
        CASE WHEN "type" = 'workflow' AND "key" = 'required_approvals'
             THEN value ~ '^\d+$' AND value::int BETWEEN 2 AND 5
             ELSE true
        END
    ) NOT VALID;

ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_max_allowed_expiry_period_days
    CHECK (
        CASE WHEN "type" = 'workflow' AND "key" = 'max_allowed_expiry_period_days'
             THEN value ~ '^\d+$' AND value::int BETWEEN 1 AND 7
             ELSE true
        END
    ) NOT VALID;

-- +goose Down
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_required_approvals;
ALTER TABLE tenant_configs DROP CONSTRAINT IF EXISTS chk_workflow_max_allowed_expiry_period_days;

ALTER TABLE tenant_configs ADD CONSTRAINT chk_workflow_minimum_approvals
    CHECK (
        CASE WHEN "type" = 'workflow' AND "key" = 'minimum_approvals'
             THEN value ~ '^\d+$' AND value::int BETWEEN 2 AND 5
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
