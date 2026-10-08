-- This migration creates the workflow_approver_groups junction table
-- After data migration (00001_wf_approver_groups_table.go), the workflow.approver_group_ids column can be dropped in a future release
-- +goose Up

CREATE TABLE IF NOT EXISTS workflow_approver_groups (
	id uuid PRIMARY KEY,
	workflow_id uuid NOT NULL,
	group_id uuid NOT NULL,
	CONSTRAINT fk_workflow_approver_groups_workflow
		FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
	CONSTRAINT fk_workflow_approver_groups_group
		FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
	CONSTRAINT unique_workflow_group UNIQUE (workflow_id, group_id)
);

CREATE INDEX idx_workflow_approver_groups_workflow_id ON workflow_approver_groups(workflow_id);
CREATE INDEX idx_workflow_approver_groups_group_id ON workflow_approver_groups(group_id);

-- +goose Down
DROP TABLE IF EXISTS workflow_approver_groups;
