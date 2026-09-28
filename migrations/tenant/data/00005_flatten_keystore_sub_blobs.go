package tenantdatamigrations

import (
	"context"
	"database/sql"
)

// upFlattenKeystoreSubBlobs populates fully-scalar rows from the still-present
// default_keystore sub-blob rows written by data migration 00004.
//
// All new rows use type = 'default_keystore' with hierarchical keys:
//
//	Source rows (type = 'default_keystore'):
//	  - key = 'locality_id'            — scalar, role_mgmt identity
//	  - key = 'common_name'            — scalar, role_mgmt identity
//	  - key = 'management_access_data' — JSON of KeystoreAccessData map
//	  - key = 'key_management_config'  — JSON of ManagementConfig{localityId, commonName, accessData}
//	  - key = 'crypto_access_data'     — JSON of map[region]CryptoConfig{subject, accessData}
//	  - key = 'supported_regions'      — JSON of []Region{name, technicalName}
//
//	Target rows (type = 'default_keystore'):
//	  role_mgmt/locality_id
//	  role_mgmt/common_name
//	  role_mgmt/access_data/<field>
//	  key_mgmt/locality_id
//	  key_mgmt/common_name
//	  key_mgmt/access_data/<field>
//	  crypto/<region>/subject
//	  crypto/<region>/access_data/<field>
//	  supported_region/<technicalName>/name
func upFlattenKeystoreSubBlobs(ctx context.Context, tx *sql.Tx) error {
	ready, err := flatRowColumnsExist(ctx, tx)
	if err != nil {
		return err
	}
	if !ready {
		return errFlatRowColumnsMissing
	}

	for _, q := range []string{
		populateRoleMgmtIdentitySQL,
		populateRoleMgmtAccessDataSQL,
		populateKeyMgmtIdentitySQL,
		populateKeyMgmtAccessDataSQL,
		populateCryptoAccessDataSQL,
		populateSupportedRegionsSQL,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func downFlattenKeystoreSubBlobs(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM tenant_configs
		WHERE "type" = 'default_keystore'
		  AND (
		    "key" LIKE 'role_mgmt/%'
		 OR "key" LIKE 'key_mgmt/%'
		 OR "key" LIKE 'crypto/%'
		 OR "key" LIKE 'supported_region/%'
		  )
	`)
	return err
}

// Copy locality_id and common_name into role_mgmt/locality_id and role_mgmt/common_name.
const populateRoleMgmtIdentitySQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT 'role_mgmt/' || "key", value_text, 'default_keystore'
FROM tenant_configs
WHERE "type" = 'default_keystore'
  AND "key" IN ('locality_id', 'common_name')
ON CONFLICT ("key") DO NOTHING
`

// Expand management_access_data JSON map into role_mgmt/access_data/<field> rows.
const populateRoleMgmtAccessDataSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT 'role_mgmt/access_data/' || ad_key, ad_value, 'default_keystore'
FROM tenant_configs,
     LATERAL jsonb_each_text(value_text::jsonb) AS kv(ad_key, ad_value)
WHERE "type" = 'default_keystore'
  AND "key" = 'management_access_data'
ON CONFLICT ("key") DO NOTHING
`

// Expand key_management_config JSON into key_mgmt/locality_id and key_mgmt/common_name.
const populateKeyMgmtIdentitySQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT 'key_mgmt/' || target_key, value_text::jsonb ->> source_key, 'default_keystore'
FROM tenant_configs,
     LATERAL (VALUES ('localityId', 'locality_id'), ('commonName', 'common_name')) AS m(source_key, target_key)
WHERE "type" = 'default_keystore'
  AND "key" = 'key_management_config'
  AND value_text::jsonb ? source_key
ON CONFLICT ("key") DO NOTHING
`

// Expand key_management_config.accessData into key_mgmt/access_data/<field> rows.
const populateKeyMgmtAccessDataSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT 'key_mgmt/access_data/' || ad_key, ad_value, 'default_keystore'
FROM tenant_configs,
     LATERAL jsonb_each_text(COALESCE(value_text::jsonb -> 'accessData', '{}')) AS kv(ad_key, ad_value)
WHERE "type" = 'default_keystore'
  AND "key" = 'key_management_config'
ON CONFLICT ("key") DO NOTHING
`

// Expand crypto_access_data JSON into crypto/<region>/subject and crypto/<region>/access_data/<field> rows.
const populateCryptoAccessDataSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT 'crypto/' || landscape || '/' || field_key, field_value, 'default_keystore'
FROM tenant_configs,
     LATERAL jsonb_each(value_text::jsonb) AS landscapes(landscape, landscape_val),
     LATERAL (
       SELECT 'subject' AS field_key, landscape_val ->> 'subject' AS field_value
         WHERE landscape_val ? 'subject'
       UNION ALL
       SELECT 'access_data/' || ad_key, ad_value
       FROM jsonb_each_text(COALESCE(landscape_val -> 'accessData', '{}')) AS a(ad_key, ad_value)
     ) AS fields(field_key, field_value)
WHERE "type" = 'default_keystore'
  AND "key" = 'crypto_access_data'
ON CONFLICT ("key") DO NOTHING
`

// Expand supported_regions JSON array into supported_region/<technicalName>/name rows.
const populateSupportedRegionsSQL = `
INSERT INTO tenant_configs ("key", value_text, "type")
SELECT 'supported_region/' || (region ->> 'technicalName') || '/name', region ->> 'name', 'default_keystore'
FROM tenant_configs,
     LATERAL jsonb_array_elements(value_text::jsonb) AS r(region)
WHERE "type" = 'default_keystore'
  AND "key" = 'supported_regions'
  AND (region ->> 'technicalName') IS NOT NULL
ON CONFLICT ("key") DO NOTHING
`
