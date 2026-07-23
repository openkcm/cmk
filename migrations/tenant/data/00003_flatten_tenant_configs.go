package tenantdatamigrations

import (
	"context"
	"database/sql"
	"errors"
)

// errFlatRowColumnsMissing fails the backfill so goose retries it rather than
// marking version 3 applied (which returning nil would do, skipping it forever).
var errFlatRowColumnsMissing = errors.New("flatten backfill needs the type and value_text columns; run schema first")

// upFlattenTenantConfigs backfills flat rows into value_text from the legacy
// WORKFLOW_CONFIG and DEFAULT_KEYSTORE blobs. Legacy blobs stay until the
// cleanup release drops them.
func upFlattenTenantConfigs(ctx context.Context, tx *sql.Tx) error {
	ready, err := flatRowColumnsExist(ctx, tx)
	if err != nil {
		return err
	}
	if !ready {
		return errFlatRowColumnsMissing
	}

	for _, q := range []string{
		flattenWorkflowConfigSQL,
		flattenKeystoreRoleManagementSQL,
		flattenKeystoreSubBlobsSQL,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func downFlattenTenantConfigs(ctx context.Context, tx *sql.Tx) error {
	ready, err := flatRowColumnsExist(ctx, tx)
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM tenant_configs WHERE length("type") > 0`)
	return err
}

// flatRowColumnsExist reports whether the flatten schema (type and value_text
// columns) is in place. Scoped to current_schema() because information_schema is
// role-scoped, not search_path-scoped, and every tenant schema has the table.
func flatRowColumnsExist(ctx context.Context, tx *sql.Tx) (bool, error) {
	var typeExists, valueTextExists bool
	err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'tenant_configs'
				AND column_name = 'type'
				AND table_schema = current_schema()
			),
			EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'tenant_configs'
				AND column_name = 'value_text'
				AND table_schema = current_schema()
			)
	`).Scan(&typeExists, &valueTextExists)
	if err != nil {
		return false, err
	}

	return typeExists && valueTextExists, nil
}

// Workflow legacy keys are PascalCase (struct has no JSON tags); target keys are snake_case.
const flattenWorkflowConfigSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT target_key, value::jsonb ->> source_key, 'workflow'
FROM tenant_configs,
     LATERAL (VALUES
        ('Enabled',                 'enabled'),
        ('MinimumApprovals',        'minimum_approvals'),
        ('RetentionPeriodDays',     'retention_period_days'),
        ('DefaultExpiryPeriodDays', 'default_expiry_period_days'),
        ('MaxExpiryPeriodDays',     'max_expiry_period_days')
     ) AS keys(source_key, target_key)
WHERE "key" = 'WORKFLOW_CONFIG' AND length("type") = 0
  AND value::jsonb ? source_key
ON CONFLICT ("key", "type") DO NOTHING
`

// Keystore identity fields (LocalityID, CommonName, AccessData) live under the
// nested roleManagementConfig sub-object. AccessData is JSON-encoded, the rest
// are scalar strings.
const flattenKeystoreRoleManagementSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT target_key,
       CASE source_key
           WHEN 'accessData' THEN (rmc -> source_key)::text
           ELSE rmc ->> source_key
       END,
       'default_keystore'
FROM tenant_configs,
     LATERAL (SELECT value::jsonb -> 'roleManagementConfig' AS rmc) AS rm,
     LATERAL (VALUES
        ('localityId', 'locality_id'),
        ('commonName', 'common_name'),
        ('accessData', 'management_access_data')
     ) AS keys(source_key, target_key)
WHERE "key" = 'DEFAULT_KEYSTORE' AND length("type") = 0
  AND rmc IS NOT NULL AND rmc ? source_key
ON CONFLICT ("key", "type") DO NOTHING
`

// Keystore sub-blobs are stored at the top level of the legacy JSON and are
// preserved verbatim as JSON-encoded flat-row values.
const flattenKeystoreSubBlobsSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT target_key, (value::jsonb -> source_key)::text, 'default_keystore'
FROM tenant_configs,
     LATERAL (VALUES
        ('keyManagementConfig', 'key_management_config'),
        ('cryptoAccessData',    'crypto_access_data'),
        ('supportedRegions',    'supported_regions')
     ) AS keys(source_key, target_key)
WHERE "key" = 'DEFAULT_KEYSTORE' AND length("type") = 0
  AND value::jsonb ? source_key
ON CONFLICT ("key", "type") DO NOTHING
`
