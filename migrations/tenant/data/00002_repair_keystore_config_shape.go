package tenantdatamigrations

import (
	"context"
	"database/sql"
)

// upRepairKeystoreConfigShape rewrites legacy DEFAULT_KEYSTORE blobs from the
// flat shape to the nested roleManagementConfig shape so the flatten backfill
// reads a canonical layout. Skips when tenant_configs is absent.
func upRepairKeystoreConfigShape(ctx context.Context, tx *sql.Tx) error {
	exists, err := tenantConfigsTableExists(ctx, tx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	_, err = tx.ExecContext(ctx, repairKeystoreConfigUpSQL)
	return err
}

// downRepairKeystoreConfigShape reverts the nested roleManagementConfig shape
// back to the flat layout. Skips when tenant_configs is absent.
func downRepairKeystoreConfigShape(ctx context.Context, tx *sql.Tx) error {
	exists, err := tenantConfigsTableExists(ctx, tx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	_, err = tx.ExecContext(ctx, repairKeystoreConfigDownSQL)
	return err
}

func tenantConfigsTableExists(ctx context.Context, tx *sql.Tx) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_name = 'tenant_configs'
			AND table_schema = current_schema()
		)
	`).Scan(&exists)
	return exists, err
}

const repairKeystoreConfigUpSQL = `
UPDATE tenant_configs
SET value = jsonb_build_object(
        'roleManagementConfig', jsonb_build_object( -- NOSONAR
            'localityId', value::jsonb -> 'localityId', -- NOSONAR
            'commonName', value::jsonb -> 'commonName', -- NOSONAR
            'accessData', value::jsonb -> 'managementAccessData'
        ),
        'supportedRegions', value::jsonb -> 'supportedRegions' -- NOSONAR
    )::jsonb
WHERE "key" = 'DEFAULT_KEYSTORE'
  AND value::jsonb ? 'localityId'
  AND NOT (value::jsonb ? 'roleManagementConfig')
`

const repairKeystoreConfigDownSQL = `
UPDATE tenant_configs
SET value = jsonb_build_object(
        'localityId',           value::jsonb -> 'roleManagementConfig' -> 'localityId',
        'commonName',           value::jsonb -> 'roleManagementConfig' -> 'commonName',
        'managementAccessData', value::jsonb -> 'roleManagementConfig' -> 'accessData',
        'supportedRegions',     value::jsonb -> 'supportedRegions'
    )::jsonb
WHERE "key" = 'DEFAULT_KEYSTORE'
  AND value::jsonb ? 'roleManagementConfig'
`
