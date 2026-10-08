package tenantdatamigrations

import (
	"context"
	"database/sql"
)

func upWorkflowApproverGroupTable(ctx context.Context, tx *sql.Tx) error {
	// Check if both the source column and target table exist
	var columnExists, tableExists bool
	err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'workflows'
				AND column_name = 'approver_group_ids'
			),
			EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_name = 'workflow_approver_groups'
			)
	`).Scan(&columnExists, &tableExists)
	if err != nil {
		return err
	}

	// If either doesn't exist, nothing to migrate
	if !columnExists || !tableExists {
		return nil
	}

	// Insert into junction table by expanding JSONB array from workflows
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflow_approver_groups (id, workflow_id, group_id)
		SELECT
			gen_random_uuid(),
			w.id as workflow_id,
			elem::uuid as group_id
		FROM workflows w,
		LATERAL jsonb_array_elements_text(w.approver_group_ids) AS elem
		WHERE w.approver_group_ids IS NOT NULL
		ON CONFLICT (workflow_id, group_id) DO NOTHING
	`)
	return err
}

func downWorkflowApproverGroupTable(ctx context.Context, tx *sql.Tx) error {
	// Check if workflow_approver_groups table exists. If not this migration is not needed
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_name = 'workflow_approver_groups'
		)
	`).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM workflow_approver_groups`)
	return err
}
