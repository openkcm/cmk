-- +goose Up
-- Rename table
ALTER TABLE workflow_approvers RENAME TO workflow_tasks;

-- Add primary key id column (split to avoid full table rewrite from volatile default)
ALTER TABLE workflow_tasks ADD COLUMN id uuid;                                         -- instant: nullable, no default
UPDATE workflow_tasks SET id = gen_random_uuid() WHERE id IS NULL;                    -- explicit backfill, no ADD COLUMN lock
ALTER TABLE workflow_tasks ALTER COLUMN id SET NOT NULL;                               -- fast: no NULLs remain
ALTER TABLE workflow_tasks DROP CONSTRAINT workflow_approvers_pkey;
ALTER TABLE workflow_tasks ADD CONSTRAINT workflow_tasks_pkey PRIMARY KEY (id);

-- Add assignee_role column
ALTER TABLE workflow_tasks ADD COLUMN assignee_role varchar(50) NOT NULL DEFAULT 'APPROVER';
ALTER TABLE workflow_tasks ADD CONSTRAINT workflow_tasks_assignee_role_check CHECK (assignee_role IN ('APPROVER', 'INITIATOR'));
-- UNIQUE across (workflow_id, user_id, assignee_role): same user can hold APPROVER and INITIATOR roles on the same workflow
ALTER TABLE workflow_tasks ADD CONSTRAINT workflow_tasks_workflow_user_role_key UNIQUE (workflow_id, user_id, assignee_role);

-- Add created_at and completed_at columns
-- Default now() back-fills existing rows at migration time (acceptable — exact time unknown)
ALTER TABLE workflow_tasks ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE workflow_tasks ADD COLUMN completed_at timestamptz NULL;

-- Rename foreign key constraint
ALTER TABLE workflow_tasks RENAME CONSTRAINT fk_workflows_approvers TO fk_workflows_tasks;

-- Create view joining workflow_tasks with workflows
CREATE VIEW workflow_task_view AS
SELECT
    wt.id,
    wt.workflow_id,
    wt.user_id,
    wt.approved,
    wt.assignee_role,
    wt.created_at,
    wt.completed_at,
    w.state          AS workflow_state,
    w.artifact_type,
    w.artifact_id,
    w.artifact_name,
    w.action_type,
    w.initiator_id,
    w.expiry_date
FROM workflow_tasks wt
JOIN workflows w ON wt.workflow_id = w.id;

-- +goose Down
DROP VIEW IF EXISTS workflow_task_view;

ALTER TABLE workflow_tasks DROP CONSTRAINT workflow_tasks_assignee_role_check;
ALTER TABLE workflow_tasks DROP CONSTRAINT workflow_tasks_pkey;
ALTER TABLE workflow_tasks DROP CONSTRAINT workflow_tasks_workflow_user_role_key;
DELETE FROM workflow_tasks WHERE assignee_role = 'INITIATOR';
ALTER TABLE workflow_tasks ADD CONSTRAINT workflow_approvers_pkey PRIMARY KEY (workflow_id, user_id);
ALTER TABLE workflow_tasks DROP COLUMN id;
ALTER TABLE workflow_tasks DROP COLUMN assignee_role;
ALTER TABLE workflow_tasks DROP COLUMN created_at;
ALTER TABLE workflow_tasks DROP COLUMN completed_at;
ALTER TABLE workflow_tasks RENAME CONSTRAINT fk_workflows_tasks TO fk_workflows_approvers;
ALTER TABLE workflow_tasks RENAME TO workflow_approvers;
