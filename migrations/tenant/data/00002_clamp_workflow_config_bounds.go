package tenantdatamigrations

import (
	"context"
	"database/sql"
)

// upClampWorkflowConfigBounds clamps any existing WORKFLOW_CONFIG rows that
// fall outside the hard limits defined in constants/workflow.go. This must
// run after schema migration 00019 adds the corresponding CHECK constraints.
func upClampWorkflowConfigBounds(ctx context.Context, tx *sql.Tx) error {
	// Guard: skip if tenant_configs or the key/value columns no longer exist.
	// Protects against future schema evolution where this table or its columns
	// may have been restructured or removed, and a fresh DB setup runs all
	// historical data migrations.
	var tableExists, keyColExists, valueColExists bool
	err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_name = 'tenant_configs'
			),
			EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'tenant_configs' AND column_name = 'key'
			),
			EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'tenant_configs' AND column_name = 'value'
			)
	`).Scan(&tableExists, &keyColExists, &valueColExists)
	if err != nil {
		return err
	}
	if !tableExists || !keyColExists || !valueColExists {
		return nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tenant_configs
		SET value = jsonb_set(
			value,
			'{minimumApprovals}',
			to_jsonb(GREATEST(2, LEAST(5, (value->>'minimumApprovals')::int)))
		)
		WHERE key = 'WORKFLOW_CONFIG'
		  AND (value->>'minimumApprovals') IS NOT NULL
		  AND (value->>'minimumApprovals')::int NOT BETWEEN 2 AND 5
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tenant_configs
		SET value = jsonb_set(
			value,
			'{retentionPeriodDays}',
			to_jsonb(GREATEST(7, LEAST(30, (value->>'retentionPeriodDays')::int)))
		)
		WHERE key = 'WORKFLOW_CONFIG'
		  AND (value->>'retentionPeriodDays') IS NOT NULL
		  AND (value->>'retentionPeriodDays')::int NOT BETWEEN 7 AND 30
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tenant_configs
		SET value = jsonb_set(
			value,
			'{maxExpiryPeriodDays}',
			to_jsonb(LEAST(7, (value->>'maxExpiryPeriodDays')::int))
		)
		WHERE key = 'WORKFLOW_CONFIG'
		  AND (value->>'maxExpiryPeriodDays') IS NOT NULL
		  AND (value->>'maxExpiryPeriodDays')::int > 7
	`)
	return err
}

func downClampWorkflowConfigBounds(_ context.Context, _ *sql.Tx) error {
	// Clamping is a one-way operation — original values are not recoverable.
	return nil
}
