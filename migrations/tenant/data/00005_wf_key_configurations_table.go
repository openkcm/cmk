package tenantdatamigrations

import (
	"context"
	"database/sql"
)

func upWorkflowKeyConfigurationsTable(ctx context.Context, tx *sql.Tx) error {
	// Check if both the target table and workflows table exist
	var tableExists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_name = 'workflow_key_configurations'
			AND table_schema = current_schema()
		)
	`).Scan(&tableExists)
	if err != nil {
		return err
	}

	if !tableExists {
		return nil
	}

	// Backfill from KEY_CONFIGURATION artifact type:
	// The artifact_id is directly the key_configuration_id
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflow_key_configurations (id, workflow_id, key_configuration_id)
		SELECT
			gen_random_uuid(),
			w.id AS workflow_id,
			w.artifact_id AS key_configuration_id
		FROM workflows w
		WHERE w.artifact_type = 'KEY_CONFIGURATION'
		ON CONFLICT (workflow_id, key_configuration_id) DO NOTHING
	`)
	if err != nil {
		return err
	}

	// Backfill from KEY artifact type:
	// Look up the key_configuration_id from the keys table via artifact_id
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflow_key_configurations (id, workflow_id, key_configuration_id)
		SELECT
			gen_random_uuid(),
			w.id AS workflow_id,
			k.key_configuration_id AS key_configuration_id
		FROM workflows w
		JOIN keys k ON k.id = w.artifact_id
		WHERE w.artifact_type = 'KEY'
		ON CONFLICT (workflow_id, key_configuration_id) DO NOTHING
	`)
	if err != nil {
		return err
	}

	// Backfill from SYSTEM artifact type, UNLINK or SWITCH action:
	// Get the current key_configuration_id from the system
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflow_key_configurations (id, workflow_id, key_configuration_id)
		SELECT
			gen_random_uuid(),
			w.id AS workflow_id,
			s.key_configuration_id AS key_configuration_id
		FROM workflows w
		JOIN systems s ON s.id = w.artifact_id
		WHERE w.artifact_type = 'SYSTEM'
		  AND w.action_type IN ('UNLINK', 'SWITCH')
		  AND s.key_configuration_id IS NOT NULL
		ON CONFLICT (workflow_id, key_configuration_id) DO NOTHING
	`)
	if err != nil {
		return err
	}

	// Backfill from SYSTEM artifact type, LINK or SWITCH action:
	// The parameters column holds the target key_configuration_id as a UUID string
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflow_key_configurations (id, workflow_id, key_configuration_id)
		SELECT
			gen_random_uuid(),
			w.id AS workflow_id,
			w.parameters::uuid AS key_configuration_id
		FROM workflows w
		WHERE w.artifact_type = 'SYSTEM'
		  AND w.action_type IN ('LINK', 'SWITCH')
		  AND EXISTS (
			SELECT 1 FROM key_configurations kc WHERE kc.id = w.parameters::uuid
		  )
		ON CONFLICT (workflow_id, key_configuration_id) DO NOTHING
	`)
	return err
}

func downWorkflowKeyConfigurationsTable(ctx context.Context, tx *sql.Tx) error {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_name = 'workflow_key_configurations'
			AND table_schema = current_schema()
		)
	`).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM workflow_key_configurations`)
	return err
}
