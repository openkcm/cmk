package tenantdatamigrations

import (
	"context"
	"database/sql"
)

// upClampWorkflowConfigBounds clamps any existing WORKFLOW_CONFIG rows that
// fall outside the hard limits defined in constants/workflow.go. This must
// run after schema migration 00019 adds the corresponding CHECK constraints.
func upClampWorkflowConfigBounds(ctx context.Context, tx *sql.Tx) error {
	// Skip unless value is still a jsonb column. The clamp casts value as jsonb,
	// which the flattened (text value) schema no longer supports; a fresh DB
	// setup replays all historical data migrations against the contracted shape.
	jsonb, err := jsonbValueColumnExists(ctx, tx)
	if err != nil {
		return err
	}
	if !jsonb {
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
