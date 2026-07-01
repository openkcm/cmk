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
SET value = (
        (value::jsonb - 'localityId' - 'commonName' - 'managementAccessData')
        || jsonb_build_object(
            'roleManagementConfig', jsonb_build_object( -- NOSONAR
                'localityId', value::jsonb -> 'localityId', -- NOSONAR
                'commonName', value::jsonb -> 'commonName', -- NOSONAR
                'accessData', value::jsonb -> 'managementAccessData'
            )
        )
    )::jsonb
WHERE "key" = 'DEFAULT_KEYSTORE'
  AND value::jsonb ? 'localityId'
  AND NOT (value::jsonb ? 'roleManagementConfig')
`

const repairKeystoreConfigDownSQL = `
UPDATE tenant_configs
SET value = (
        (value::jsonb - 'roleManagementConfig')
        || jsonb_build_object(
            'localityId', value::jsonb -> 'roleManagementConfig' -> 'localityId', -- NOSONAR
            'commonName', value::jsonb -> 'roleManagementConfig' -> 'commonName'
        )
        || CASE
               WHEN jsonb_typeof(value::jsonb -> 'roleManagementConfig' -> 'accessData') IS NULL
                    OR jsonb_typeof(value::jsonb -> 'roleManagementConfig' -> 'accessData') = 'null'
               THEN '{}'::jsonb
               ELSE jsonb_build_object('managementAccessData', value::jsonb -> 'roleManagementConfig' -> 'accessData')
           END
    )::jsonb
WHERE "key" = 'DEFAULT_KEYSTORE'
  AND value::jsonb ? 'roleManagementConfig'
`
