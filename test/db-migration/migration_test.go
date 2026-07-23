package dbmigration_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

type GooseVersion struct {
	Version int64 `gorm:"column:version_id"`
}

type ColumnInfo struct {
	TableName              string `gorm:"column:table_name"`
	ColumnName             string `gorm:"column:column_name"`
	DataType               string `gorm:"column:data_type"`
	IsNullable             string `gorm:"column:is_nullable"`
	CharacterMaximumLength *int64 `gorm:"column:character_maximum_length"`
}

type SchemaMigrationSetup struct {
	Target  db.MigrationTarget
	Version *int64
}

type DataMigrationSetup struct {
	Target        db.MigrationTarget
	Version       int64
	SchemaVersion *int64
}

var ErrMigrationFail = errors.New("migration didnt work as expected")

func setupSchemaMigration(t *testing.T, migration SchemaMigrationSetup) (*multitenancy.DB, db.Migrator, string) {
	t.Helper()

	var dbCon *multitenancy.DB
	var dbCfg config.Database
	var tenants []string

	if migration.Target == db.SharedTarget {
		dbCon, _, dbCfg = testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
			SharedVersion:  migration.Version,
		}, testutils.WithGenerateTenants(0))
		tenants = []string{db.SharedSchema}
	} else {
		dbCon, tenants, dbCfg = testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
			TenantVersion:  migration.Version,
		})
	}

	r := sql.NewRepository(dbCon)
	m, err := db.NewMigrator(r, &config.Config{Database: dbCfg})
	assert.NoError(t, err)

	return dbCon, m, tenants[0]
}

func setupDataMigration(t *testing.T, migration DataMigrationSetup) (*multitenancy.DB, db.Migrator, string) {
	t.Helper()
	dbCon, m, tenant := setupSchemaMigration(t, SchemaMigrationSetup{
		Target:  migration.Target,
		Version: migration.SchemaVersion,
	})

	if migration.Version != 0 {
		_, err := m.MigrateTo(t.Context(), db.Migration{
			Type:   db.DataMigration,
			Target: migration.Target,
		}, migration.Version)
		assert.NoError(t, err)
	}

	return dbCon, m, tenant
}

func assertVersion(t *testing.T, dbCon *multitenancy.DB, version int64, versionTable string, tenant string) {
	t.Helper()

	var res GooseVersion
	query := fmt.Sprintf("SELECT version_id FROM %s ORDER BY id DESC", versionTable)

	err := dbCon.WithTenant(t.Context(), tenant, func(tx *multitenancy.DB) error {
		return tx.Raw(query).Scan(&res).Error
	})

	assert.NoError(t, err)

	assert.Equal(t, version, res.Version)
}

func TestMissingSchemaScripts(t *testing.T) {
	gooseMigrated, gooseTenant, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})

	// There is no current support to create a tenant if shared version is set to 0
	// due to missing tenant table. Tenant must be created on a different step
	gormMigrated, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		SharedVersion:  ptr.PointTo(int64(0)),
		TenantVersion:  ptr.PointTo(int64(0)),
	}, testutils.WithGenerateTenants(0))

	assert.NoError(t, gormMigrated.MigrateSharedModels(t.Context()))
	gormTenant := testutils.NewTenant(func(_ *model.Tenant) {})
	testutils.CreateDBTenant(t, gormMigrated, gormTenant)
	err := gormMigrated.RegisterModels(
		t.Context(),
		&model.KeyConfiguration{},
		&model.Key{},
		&model.KeyVersion{},
		&model.KeyLabel{},
		&model.System{},
		&model.SystemProperty{},
		&model.Workflow{},
		&model.WorkflowApprover{},
		&model.Tenant{},
		&model.TenantConfig{},
		&model.Certificate{},
		&model.Group{},
		&model.Tag{},
		&model.ImportParams{},
		&model.Keystore{},
		&model.Event{},
	)
	assert.NoError(t, err)
	assert.NoError(t, gormMigrated.MigrateTenantModels(t.Context(), gormTenant.SchemaName))

	// This test is useful to remember the dev to create migration scripts
	// whenever models are updated as it this test will otherwise fail
	t.Run("Should fail if migrations outcome differ from auto", func(t *testing.T) {
		query := `SELECT table_name,
			column_name,
			data_type,
			is_nullable,
			character_maximum_length
		FROM information_schema.columns
		WHERE (table_schema = 'public' OR table_schema = ?) AND table_name NOT LIKE '%goose%'
		ORDER BY table_name, ordinal_position`

		var gooseRows []ColumnInfo
		err := gooseMigrated.Raw(query, gooseTenant[0]).Scan(&gooseRows).Error
		assert.NoError(t, err)

		var gormRows []ColumnInfo
		err = gormMigrated.Raw(query, gormTenant.SchemaName).Scan(&gormRows).Error
		assert.NoError(t, err)

		assert.Subset(t, gooseRows, gormRows)
	})
}

func TestSchemaMigrations(t *testing.T) {
	tests := []struct {
		name            string
		target          db.MigrationTarget
		downgrade       bool
		version         int64
		setupData       func(t *testing.T) func(db *multitenancy.DB) error
		assertMigration func(t *testing.T) func(db *multitenancy.DB) error
	}{
		{
			name:      "Should up shared/00001_init_shared.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   1,
		},
		{
			name:      "Should down shared/00001_init_shared.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   1,
		},
		{
			name:      "Should up shared/00002_rename_keystore_pool_table.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   2,
		},
		{
			name:      "Should down shared/00002_rename_keystore_pool_table.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   2,
		},
		{
			name:      "Should up shared/00003_add_tenant_name.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   3,
		},
		{
			name:      "Should down shared/00003_add_tenant_name.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   3,
		},
		{
			name:      "Should up shared/00004_delete_tenant_region.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   4,
		},
		{
			name:      "Should down shared/00004_delete_tenant_region.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   4,
		},
		{
			name:      "Should up shared/00005_create_orbital_schema.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   5,
			assertMigration: func(t *testing.T) func(con *multitenancy.DB) error {
				t.Helper()
				return func(con *multitenancy.DB) error {
					var exists bool
					err := con.Raw(`
						SELECT EXISTS (
							SELECT 1
							FROM pg_namespace
							WHERE nspname = ?
						)
						`, "orbital").Scan(&exists).Error
					assert.NoError(t, err)
					assert.True(t, exists)

					return nil
				}
			},
		},
		{
			name:      "Should down shared/00005_create_orbital_schema.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   5,
		},

		{
			name:      "Should up tenant/00001_init_shared.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   1,
		},
		{
			name:      "Should down tenant/00001_init_shared.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   1,
		},
		{
			name:      "Should up tenant/00002_create_tag_table.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   2,
		},
		{
			name:      "Should down tenant/00002_create_tag_table.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   2,
		},
		{
			name:      "Should up tenant/00003_add_error_event_table.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   3,
		},
		{
			name:      "Should down tenant/00003_add_error_event_table.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   3,
		},
		{
			name:      "Should up tenant/00004_rename_group_table_expand.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   4,
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()
				return func(db *multitenancy.DB) error {
					// Verify Insert
					id := uuid.NewString()
					res := db.Exec(`INSERT INTO "groups" (id, "name", description, "role", iam_identifier) VALUES(?, ?, ?, ?, ?)`,
						id, uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))

					var count int
					err := db.Raw(`SELECT COUNT(*) FROM "group" WHERE id = ?`, id).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 1, count)

					// Verify Update
					newName := uuid.NewString()
					res = db.Exec(`UPDATE "groups" SET "name" = ?`, newName)
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))

					err = db.Raw(`SELECT COUNT(*) FROM "group" WHERE id = ? AND "name" = ?`, id, newName).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 1, count)

					// Verify Delete
					res = db.Exec(`DELETE FROM "groups" WHERE id = ?`, id)
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))

					err = db.Raw(`SELECT COUNT(*) FROM "group" WHERE id = ?`, id).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 0, count)

					return nil
				}
			},
		},
		{
			name:      "Should down tenant/00004_rename_group_table_expand.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   4,
		},
		{
			name:      "Should up tenant/00005_fix_error_event_columns.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   5,
		},
		{
			name:      "Should down tenant/00005_fix_error_event_columns.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   5,
		},
		{
			name:      "Should up tenant/00006_refactor_key_version.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   6,
		},
		{
			name:      "Should down tenant/00006_refactor_key_version.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   6,
		},
		{
			name:      "Should up tenant/00007_delete_user_names.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   7,
		},
		{
			name:      "Should down tenant/00007_delete_user_names.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   7,
		},
		{
			name:      "Should up tenant/00008_add_minimum_approval_count.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   8,
		},
		{
			name:      "Should down tenant/00008_add_minimum_approval_count.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   8,
		},
		{
			name:      "Should up tenant/00009_add_under_workflow_column_to_system.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   9,
		},
		{
			name:      "Should down tenant/00009_add_under_workflow_column_to_system.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   9,
		},
		{
			name:      "Should up tenant/00010_add_wf_approver_groups_table.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   10,
		},
		{
			name:      "Should down tenant/00010_add_wf_approver_groups_table.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   10,
		},
		{
			name:      "Should up tenant/00011_delete_key_primary.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   11,
		},
		{
			name:      "Should down tenant/00011_delete_key_primary.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   11,
		},
		{
			name:      "Should up tenant/00012_add_enum_check_constraints.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   12,
			assertMigration: func(t *testing.T) func(con *multitenancy.DB) error {
				t.Helper()
				return func(con *multitenancy.DB) error {
					cases := []struct {
						name string
						sql  string
					}{
						{
							name: "workflows.state",
							sql:  `INSERT INTO workflows (created_at, updated_at, id, state, initiator_id, artifact_type, artifact_id, action_type) VALUES (now(), now(), gen_random_uuid(), 'BOGUS', 'u', 'KEY', gen_random_uuid(), 'DELETE')`,
						},
						{
							name: "workflows.action_type",
							sql:  `INSERT INTO workflows (created_at, updated_at, id, state, initiator_id, artifact_type, artifact_id, action_type) VALUES (now(), now(), gen_random_uuid(), 'INITIAL', 'u', 'KEY', gen_random_uuid(), 'BOGUS')`,
						},
						{
							name: "workflows.artifact_type",
							sql:  `INSERT INTO workflows (created_at, updated_at, id, state, initiator_id, artifact_type, artifact_id, action_type) VALUES (now(), now(), gen_random_uuid(), 'INITIAL', 'u', 'BOGUS', gen_random_uuid(), 'DELETE')`,
						},
						{
							name: "keys.state",
							sql:  `INSERT INTO keys (created_at, updated_at, id, key_configuration_id, name, key_type, algorithm, provider, region, state) VALUES (now(), now(), gen_random_uuid(), '00000000-0000-0000-0000-000000000001'::uuid, 'n', 't', 'a', 'p', 'r', 'BOGUS')`,
						},
						{
							name: "systems.status",
							sql:  `INSERT INTO systems (id, identifier, region, type, status) VALUES (gen_random_uuid(), 'i', 'r', 't', 'BOGUS')`,
						},
					}
					for _, c := range cases {
						// Wrap each insert in its own transaction so a CHECK
						// violation rolls back without aborting the others.
						err := con.Transaction(func(tx *multitenancy.DB) error {
							return tx.Exec(c.sql).Error
						})
						assert.ErrorContains(t, err, "violates check constraint",
							"%s: insert with invalid value should be rejected by CHECK", c.name)
					}
					return nil
				}
			},
		},
		{
			name:      "Should down tenant/00012_add_enum_check_constraints.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   12,
		},
		{
			name:      "Should up tenant/00013_add_under_workflow_column_to_key.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   13,
		},
		{
			name:      "Should down tenant/00013_add_under_workflow_column_to_key.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   13,
		},
		{
			name:      "Should up tenant/00014_add_system_target_keyconfig.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   14,
		},
		{
			name:      "Should down tenant/00014_add_system_target_keyconfig.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   14,
		},
		{
			name:      "Should up tenant/00015_flatten_tenant_configs.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   15,
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var typeExists, valueTextExists bool
					err := db.Raw(`
						SELECT
							EXISTS (SELECT 1 FROM information_schema.columns
								WHERE table_name = 'tenant_configs' AND column_name = 'type'),
							EXISTS (SELECT 1 FROM information_schema.columns
								WHERE table_name = 'tenant_configs' AND column_name = 'value_text')
					`).Row().Scan(&typeExists, &valueTextExists)
					assert.NoError(t, err)
					assert.True(t, typeExists, "type column must be added")
					assert.True(t, valueTextExists, "value_text column must be added")

					return nil
				}
			},
		},
		{
			name:      "Should down tenant/00015_flatten_tenant_configs.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   15,
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var valueTextExists bool
					err := db.Raw(`
						SELECT EXISTS (SELECT 1 FROM information_schema.columns
							WHERE table_name = 'tenant_configs' AND column_name = 'value_text')
					`).Row().Scan(&valueTextExists)
					assert.NoError(t, err)
					assert.False(t, valueTextExists, "value_text column must be dropped")

					return nil
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := db.Migration{
				Downgrade: tt.downgrade,
				Type:      db.SchemaMigration,
				Target:    tt.target,
			}

			var setupVersion int64
			if tt.downgrade {
				setupVersion = tt.version
			} else {
				setupVersion = tt.version - 1
			}

			dbCon, m, tenant := setupSchemaMigration(t, SchemaMigrationSetup{
				Target:  tt.target,
				Version: ptr.PointTo(setupVersion),
			})

			if tt.setupData != nil {
				err := dbCon.WithTenant(t.Context(), tenant, tt.setupData(t))
				assert.NoError(t, err)
			}

			var migrateVersion int64
			if tt.downgrade {
				migrateVersion = tt.version - 1
			} else {
				migrateVersion = tt.version
			}

			_, err := m.MigrateTo(t.Context(), migration, migrateVersion)
			assert.NoError(t, err)

			assertVersion(t, dbCon, migrateVersion, db.SchemaMigrationTable, tenant)
			if tt.assertMigration != nil {
				err := dbCon.WithTenant(t.Context(), tenant, tt.assertMigration(t))
				assert.NoError(t, err)
			}
		})
	}
}

func TestDataMigrations(t *testing.T) {
	tests := []struct {
		name            string
		target          db.MigrationTarget
		version         int64
		schemaVersion   *int64
		assertMigration func(t *testing.T) func(db *multitenancy.DB) error
		setupData       func(t *testing.T) func(db *multitenancy.DB) error
		downgrade       bool
	}{
		{
			name:          "Should skip data migration if workflow approvers column does not exists",
			target:        db.TenantTarget,
			version:       1,
			schemaVersion: ptr.PointTo(int64(9)),
		},
		{
			name:          "Should migrate up workflow approvers to workflow_approver_groups table",
			target:        db.TenantTarget,
			version:       1,
			schemaVersion: ptr.PointTo(int64(10)),
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var count int

					err := db.Raw(`SELECT COUNT(*) FROM workflow_approver_groups`).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 3, count)

					var workflow1ID string
					err = db.Raw(`SELECT id FROM workflows WHERE initiator_id = 'user-1'`).Scan(&workflow1ID).Error
					assert.NoError(t, err)

					err = db.Raw(`SELECT COUNT(*) FROM workflow_approver_groups WHERE workflow_id = ?`, workflow1ID).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 2, count)

					var workflow2ID string
					err = db.Raw(`SELECT id FROM workflows WHERE initiator_id = 'user-2'`).Scan(&workflow2ID).Error
					assert.NoError(t, err)

					err = db.Raw(`SELECT COUNT(*) FROM workflow_approver_groups WHERE workflow_id = ?`, workflow2ID).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 1, count)

					var workflow3ID string
					err = db.Raw(`SELECT id FROM workflows WHERE initiator_id = 'user-3'`).Scan(&workflow3ID).Error
					assert.NoError(t, err)

					err = db.Raw(`SELECT COUNT(*) FROM workflow_approver_groups WHERE workflow_id = ?`, workflow3ID).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 0, count)

					return nil
				}
			},
			setupData: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					groupID1 := uuid.New()
					groupID2 := uuid.New()
					groupID3 := uuid.New()
					groups := []*model.Group{
						testutils.NewGroup(func(g *model.Group) {
							g.ID = groupID1
						}),
						testutils.NewGroup(func(g *model.Group) {
							g.ID = groupID2
						}),
						testutils.NewGroup(func(g *model.Group) {
							g.ID = groupID3
						}),
					}

					for _, g := range groups {
						err := db.Create(g).Error
						assert.NoError(t, err)
					}

					wfs := []*model.Workflow{
						testutils.NewWorkflow(func(w *model.Workflow) {
							w.ApproverGroupIDs = json.RawMessage(fmt.Sprintf(`["%s", "%s"]`, groupID1, groupID2))
							w.InitiatorID = "user-1"
						}),
						testutils.NewWorkflow(func(w *model.Workflow) {
							w.ApproverGroupIDs = json.RawMessage(fmt.Sprintf(`["%s"]`, groupID3))
							w.InitiatorID = "user-2"
						}),
						testutils.NewWorkflow(func(w *model.Workflow) {
							w.InitiatorID = "user-3"
						}),
					}

					for _, w := range wfs {
						err := db.Create(w).Error
						assert.NoError(t, err)
					}

					return nil
				}
			},
		},
		{
			name:          "Should migrate down 0001",
			target:        db.TenantTarget,
			version:       1,
			schemaVersion: ptr.PointTo(int64(10)),
			downgrade:     true,
		},
		{
			name:          "Should repair keystore config shape into nested roleManagementConfig",
			target:        db.TenantTarget,
			version:       2,
			schemaVersion: ptr.PointTo(int64(15)),
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var locality string
					err := db.Raw(
						`SELECT value::jsonb -> 'roleManagementConfig' ->> 'localityId'
						 FROM tenant_configs WHERE "key" = 'DEFAULT_KEYSTORE'`,
					).Scan(&locality).Error
					assert.NoError(t, err)
					assert.Equal(t, "loc-1", locality)

					var hasLegacyShape bool
					err = db.Raw(
						`SELECT value::jsonb ? 'localityId' FROM tenant_configs WHERE "key" = 'DEFAULT_KEYSTORE'`,
					).Scan(&hasLegacyShape).Error
					assert.NoError(t, err)
					assert.False(t, hasLegacyShape, "flat keystore shape must be rewritten to nested")

					return nil
				}
			},
			setupData: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					return db.Exec(
						`INSERT INTO tenant_configs ("key", value, "type") VALUES
							('DEFAULT_KEYSTORE', '{"localityId":"loc-1","commonName":"cn-1"}', '')`,
					).Error
				}
			},
		},
		{
			name:          "Should flatten tenant_configs legacy blobs into typed flat rows",
			target:        db.TenantTarget,
			version:       3,
			schemaVersion: ptr.PointTo(int64(15)),
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var count int

					err := db.Raw(
						`SELECT COUNT(*) FROM tenant_configs WHERE "type" = 'workflow'`,
					).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 5, count, "all 5 workflow keys must be flattened")

					var enabled string
					err = db.Raw(
						`SELECT value_text FROM tenant_configs WHERE "type" = 'workflow' AND "key" = 'enabled'`,
					).Scan(&enabled).Error
					assert.NoError(t, err)
					assert.Equal(t, "true", enabled)

					var minApprovals string
					err = db.Raw(
						`SELECT value_text FROM tenant_configs WHERE "type" = 'workflow' AND "key" = 'minimum_approvals'`,
					).Scan(&minApprovals).Error
					assert.NoError(t, err)
					assert.Equal(t, "2", minApprovals)

					err = db.Raw(
						`SELECT COUNT(*) FROM tenant_configs WHERE "type" = 'default_keystore'`,
					).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 2, count, "locality_id and common_name must be flattened")

					var locality string
					err = db.Raw(
						`SELECT value_text FROM tenant_configs WHERE "type" = 'default_keystore' AND "key" = 'locality_id'`,
					).Scan(&locality).Error
					assert.NoError(t, err)
					assert.Equal(t, "loc-1", locality)

					// Legacy blobs are preserved as a read-time fallback for unmigrated tenants.
					err = db.Raw(
						`SELECT COUNT(*) FROM tenant_configs WHERE length("type") = 0`,
					).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 2, count, "legacy blobs must remain")

					return nil
				}
			},
			setupData: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					return db.Exec(
						`INSERT INTO tenant_configs ("key", value, "type") VALUES
							('WORKFLOW_CONFIG', '{"Enabled":true,"MinimumApprovals":2,"RetentionPeriodDays":30,"DefaultExpiryPeriodDays":7,"MaxExpiryPeriodDays":14}', ''),
							('DEFAULT_KEYSTORE', '{"roleManagementConfig":{"localityId":"loc-1","commonName":"cn-1"}}', '')`,
					).Error
				}
			},
		},
		{
			name:          "Should migrate down repair keystore config shape",
			target:        db.TenantTarget,
			version:       2,
			schemaVersion: ptr.PointTo(int64(15)),
			downgrade:     true,
			setupData: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					return db.Exec(
						`INSERT INTO tenant_configs ("key", value, "type") VALUES
							('DEFAULT_KEYSTORE', '{"roleManagementConfig":{"localityId":"loc-1","commonName":"cn-1"}}', '')`,
					).Error
				}
			},
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var locality string
					err := db.Raw(
						`SELECT value::jsonb ->> 'localityId' FROM tenant_configs WHERE "key" = 'DEFAULT_KEYSTORE'`,
					).Scan(&locality).Error
					assert.NoError(t, err)
					assert.Equal(t, "loc-1", locality, "repair down must restore the flat keystore shape")

					return nil
				}
			},
		},
		{
			name:          "Should migrate down flatten tenant_configs",
			target:        db.TenantTarget,
			version:       3,
			schemaVersion: ptr.PointTo(int64(15)),
			downgrade:     true,
			setupData: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					return db.Exec(
						`INSERT INTO tenant_configs ("key", value_text, "type") VALUES
							('enabled', 'true', 'workflow'),
							('minimum_approvals', '2', 'workflow')`,
					).Error
				}
			},
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()

				return func(db *multitenancy.DB) error {
					var count int
					err := db.Raw(
						`SELECT COUNT(*) FROM tenant_configs WHERE length("type") > 0`,
					).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 0, count, "flatten down must remove flat rows")

					return nil
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := db.Migration{
				Downgrade: tt.downgrade,
				Type:      db.DataMigration,
				Target:    tt.target,
			}

			var setupVersion int64
			if tt.downgrade {
				setupVersion = tt.version
			} else {
				setupVersion = tt.version - 1
			}

			dbCon, m, tenant := setupDataMigration(t, DataMigrationSetup{
				Target:        tt.target,
				SchemaVersion: tt.schemaVersion,
				Version:       setupVersion,
			})

			if tt.setupData != nil {
				err := dbCon.WithTenant(t.Context(), tenant, tt.setupData(t))
				assert.NoError(t, err)
			}

			var migrateVersion int64
			if tt.downgrade {
				migrateVersion = tt.version - 1
			} else {
				migrateVersion = tt.version
			}

			_, err := m.MigrateTo(t.Context(), migration, migrateVersion)
			assert.NoError(t, err)

			assertVersion(t, dbCon, migrateVersion, db.DataMigrationTable, tenant)
			if tt.assertMigration != nil {
				err := dbCon.WithTenant(t.Context(), tenant, tt.assertMigration(t))
				assert.NoError(t, err)
			}
		})
	}
}

// TestFlattenBackfillFailsWhenSchemaMissing verifies the backfill errors (rather
// than no-ops) when run before the flatten schema, so goose retries it instead of
// marking it applied and skipping the backfill permanently.
func TestFlattenBackfillFailsWhenSchemaMissing(t *testing.T) {
	dbCon, m, tenant := setupDataMigration(t, DataMigrationSetup{
		Target:        db.TenantTarget,
		SchemaVersion: ptr.PointTo(int64(14)), // before flatten schema (00015)
		Version:       2,                      // repair applied, flatten (3) pending
	})

	err := dbCon.WithTenant(t.Context(), tenant, func(db *multitenancy.DB) error {
		return db.Exec(
			`INSERT INTO tenant_configs ("key", value) VALUES
				('WORKFLOW_CONFIG', '{"Enabled":true,"MinimumApprovals":2,"RetentionPeriodDays":30,"DefaultExpiryPeriodDays":7,"MaxExpiryPeriodDays":14}')`,
		).Error
	})
	assert.NoError(t, err)

	_, err = m.MigrateTo(t.Context(), db.Migration{
		Type:   db.DataMigration,
		Target: db.TenantTarget,
	}, 3)
	assert.Error(t, err, "backfill must fail when the flatten schema is absent")

	// Version 3 must remain unapplied so it retries after the schema migration.
	assertVersion(t, dbCon, 2, db.DataMigrationTable, tenant)
}

// TestEnumCheckConstraintDrift asserts the IN(...) lists in migration 00012
// match the Go enum slices for workflow types. KeyState and SystemStatus are
// generated from the OpenAPI spec via oapi-codegen and not duplicated here.
func TestEnumCheckConstraintDrift(t *testing.T) {
	const migrationFile = "../../migrations/tenant/schema/00012_add_enum_check_constraints.sql"

	abs, err := filepath.Abs(migrationFile)
	require.NoError(t, err)

	data, err := os.ReadFile(abs)
	require.NoError(t, err, "failed to read migration file %s", abs)
	sql := string(data)

	cases := []struct {
		name       string
		constraint string
		want       []string
	}{
		{
			name:       "workflows.state",
			constraint: "chk_workflows_state",
			want:       toStrings(model.WorkflowStates),
		},
		{
			name:       "workflows.action_type",
			constraint: "chk_workflows_action_type",
			want:       toStrings(model.WorkflowActionTypes),
		},
		{
			name:       "workflows.artifact_type",
			constraint: "chk_workflows_artifact_type",
			want:       toStrings(model.WorkflowArtifactTypes),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCheckValues(t, sql, tc.constraint)
			assert.ElementsMatch(t, tc.want, got,
				"CHECK constraint %s drifted from Go enum slice", tc.constraint)
		})
	}
}

// extractCheckValues pulls the string literals out of
// `ADD CONSTRAINT <name> CHECK (... IN (...))`.
func extractCheckValues(t *testing.T, sql, constraint string) []string {
	t.Helper()
	pattern := `ADD CONSTRAINT ` + regexp.QuoteMeta(constraint) + `\s+CHECK\s*\([^)]*IN\s*\(([^)]+)\)`
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(sql)
	require.NotNil(t, m, "constraint %s not found in migration SQL", constraint)

	literal := regexp.MustCompile(`'([^']*)'`)
	matches := literal.FindAllStringSubmatch(m[1], -1)

	values := make([]string, 0, len(matches))
	for _, lm := range matches {
		values = append(values, strings.TrimSpace(lm[1]))
	}
	return values
}

func toStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
