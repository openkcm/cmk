package tenantdatamigrations

import (
	"context"
	"database/sql"
)

func upRemoveLegacyTenantConfigBlobs(ctx context.Context, tx *sql.Tx) error {
	// Delete legacy JSON blobs after 00002 backfilled them into flat rows.
	_, err := tx.ExecContext(ctx, `
		DELETE FROM tenant_configs
		WHERE length("type") = 0
		  AND "key" IN ('WORKFLOW_CONFIG', 'DEFAULT_KEYSTORE')
	`)
	return err
}

// downRemoveLegacyTenantConfigBlobs is a no-op: rolling back requires
// reverting the flatten schema change first, at which point the flat rows
// that would reseed the blobs are already gone.
func downRemoveLegacyTenantConfigBlobs(_ context.Context, _ *sql.Tx) error {
	return nil
}
