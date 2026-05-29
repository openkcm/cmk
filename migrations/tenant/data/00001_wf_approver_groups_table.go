package tenantdatamigrations

import (
	"context"
	"database/sql"
)

func upWorkflowApproverGroupTable(ctx context.Context, tx *sql.Tx) error {
	// Insert into junction table by expanding JSONB array from workflows
	_, err := tx.ExecContext(ctx, `
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
	_, err := tx.ExecContext(ctx, `DELETE FROM workflow_approver_groups`)
	return err
}
