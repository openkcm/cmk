package tenantdatamigrations

import (
	"context"
	"database/sql"
)

// upRenameWorkflowConfigKeys renames flat workflow config keys to match the
// updated WorkflowConfig model fields (internal/model/tenantconfigs.go).
//
// Renames (type='workflow' rows in tenant_configs):
//
//	minimum_approvals         → required_approvals + min_allowed_approvals
//	max_approvals             → max_allowed_approvals
//	min_retention_period_days → min_allowed_retention_period_days
//	max_retention_period_days → max_allowed_retention_period_days
//	default_expiry_period_days → expiry_period_days
//	max_expiry_period_days    → max_allowed_expiry_period_days
func upRenameWorkflowConfigKeys(ctx context.Context, tx *sql.Tx) error {
	renames := [][2]string{
		{"max_approvals", "max_allowed_approvals"},
		{"min_retention_period_days", "min_allowed_retention_period_days"},
		{"max_retention_period_days", "max_allowed_retention_period_days"},
		{"default_expiry_period_days", "expiry_period_days"},
		{"max_expiry_period_days", "max_allowed_expiry_period_days"},
	}

	for _, pair := range renames {
		if _, err := tx.ExecContext(ctx, `
			UPDATE tenant_configs SET "key" = $1
			WHERE "type" = 'workflow' AND "key" = $2
		`, pair[1], pair[0]); err != nil {
			return err
		}
	}

	// minimum_approvals → required_approvals (rename) + min_allowed_approvals (insert copy)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_configs ("type", "key", value)
		SELECT "type", 'min_allowed_approvals', value
		FROM tenant_configs
		WHERE "type" = 'workflow' AND "key" = 'minimum_approvals'
		ON CONFLICT DO NOTHING
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE tenant_configs SET "key" = 'required_approvals'
		WHERE "type" = 'workflow' AND "key" = 'minimum_approvals'
	`); err != nil {
		return err
	}

	return nil
}

func downRenameWorkflowConfigKeys(ctx context.Context, tx *sql.Tx) error {
	renames := [][2]string{
		{"max_allowed_approvals", "max_approvals"},
		{"min_allowed_retention_period_days", "min_retention_period_days"},
		{"max_allowed_retention_period_days", "max_retention_period_days"},
		{"expiry_period_days", "default_expiry_period_days"},
		{"max_allowed_expiry_period_days", "max_expiry_period_days"},
	}

	for _, pair := range renames {
		if _, err := tx.ExecContext(ctx, `
			UPDATE tenant_configs SET "key" = $1
			WHERE "type" = 'workflow' AND "key" = $2
		`, pair[1], pair[0]); err != nil {
			return err
		}
	}

	// required_approvals → minimum_approvals; delete the copied min_allowed_approvals row
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM tenant_configs
		WHERE "type" = 'workflow' AND "key" = 'min_allowed_approvals'
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE tenant_configs SET "key" = 'minimum_approvals'
		WHERE "type" = 'workflow' AND "key" = 'required_approvals'
	`); err != nil {
		return err
	}

	return nil
}
