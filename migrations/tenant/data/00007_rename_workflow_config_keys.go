package tenantdatamigrations

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/openkcm/cmk/internal/constants"
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
//
// Any renamed key whose value is '0' is set to its default so the application
// never reads a zero that would fail validation.
func upRenameWorkflowConfigKeys(ctx context.Context, tx *sql.Tx) error {
	renames := [][2]string{
		{"max_approvals", "max_allowed_approvals"},
		{"min_retention_period_days", "min_allowed_retention_period_days"},
		{"max_retention_period_days", "max_allowed_retention_period_days"},
		{"default_expiry_period_days", "expiry_period_days"},
		{"max_expiry_period_days", "max_allowed_expiry_period_days"},
		{"minimum_approvals", "required_approvals"},
	}

	for _, pair := range renames {
		if _, err := tx.ExecContext(ctx, `
			UPDATE tenant_configs SET "key" = $1
			WHERE "type" = 'workflow' AND "key" = $2
		`, pair[1], pair[0]); err != nil {
			return err
		}
	}

	// Set defaults for any key whose value is '0' after the rename.
	defaults := [][2]string{
		{"required_approvals", strconv.Itoa(constants.MinAllowedApprovals)},
		{"min_allowed_approvals", strconv.Itoa(constants.MinAllowedApprovals)},
		{"max_allowed_approvals", strconv.Itoa(constants.MaxAllowedApprovals)},
		{"retention_period_days", strconv.Itoa(constants.DefaultRetentionPeriodDays)},
		{"min_allowed_retention_period_days", strconv.Itoa(constants.MinRetentionPeriodDays)},
		{"max_allowed_retention_period_days", strconv.Itoa(constants.MaxRetentionPeriodDays)},
		{"expiry_period_days", strconv.Itoa(constants.DefaultExpiryPeriodDays)},
		{"max_allowed_expiry_period_days", strconv.Itoa(constants.MaxAllowedExpiryPeriodDays)},
	}

	// Ensure every key exists with a valid value: insert the default if the row
	// is missing, or fix it if it was stored as '0'.
	for _, pair := range defaults {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tenant_configs ("type", "key", value)
			VALUES ('workflow', $2, $1)
			ON CONFLICT ("key", "type") DO UPDATE SET value = $1
			WHERE tenant_configs.value = '0'
		`, pair[1], pair[0]); err != nil {
			return err
		}
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
		{"required_approvals", "minimum_approvals"},
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

	return nil
}
