package tenantdatamigrations

import (
	"context"
	"database/sql"
)

// upCopyGroupToGroups should only be ran in between schema changes
// to rename group table to groups in a 2 Step Manner.
// If the migrate up for schema ran the destroy before the data migrations
// are executed, this data migration should simply have no effects
func upCopyGroupToGroups(ctx context.Context, tx *sql.Tx) error {
	var migrate bool
	query := `
        SELECT EXISTS (
            SELECT 1
            FROM information_schema.tables
            WHERE table_name = 'group'
        )
	`
	err := tx.QueryRowContext(ctx, query).Scan(&migrate)
	if err != nil {
		return err
	}

	if !migrate {
		return nil
	}

	query = `
		INSERT INTO "groups" (id, name, description, role, iam_identifier)
		SELECT id, name, description, role, iam_identifier
		FROM "group"
		ON CONFLICT (id) DO NOTHING;
	`
	_, err = tx.ExecContext(ctx, query)
	return err
}

func downCopyGroupToGroups(ctx context.Context, tx *sql.Tx) error {
	// This is not needed as this migration only copies data from one table to another
	return nil
}
