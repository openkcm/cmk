package dbmigration_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

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
		CreateDatabase:      true,
		WithIsolatedService: true,
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
		err := gooseMigrated.Raw(query, gooseTenant).Scan(&gooseRows).Error
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
			name:      "Should up tenant/00003_create_group_table.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   3,
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()
				return func(db *multitenancy.DB) error {
					// Verify Insert
					id := uuid.NewString()
					res := db.Exec(`INSERT INTO "group" (id, "name", description, "role", iam_identifier) VALUES(?, ?, ?, ?, ?)`,
						id, uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))

					var count int
					err := db.Raw(`SELECT COUNT(*) FROM "groups" WHERE id = ?`, id).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 1, count)

					// Verify Update
					newName := uuid.NewString()
					res = db.Exec(`UPDATE "group" SET "name" = ?`, newName)
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))

					err = db.Raw(`SELECT COUNT(*) FROM "groups" WHERE id = ? AND "name" = ?`, id, newName).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 1, count)

					// Verify Delete
					res = db.Exec(`DELETE FROM "group" WHERE id = ?`, id)
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))

					err = db.Raw(`SELECT COUNT(*) FROM "groups" WHERE id = ?`, id).Scan(&count).Error
					assert.NoError(t, err)
					assert.Equal(t, 0, count)

					return nil
				}
			},
		},
		{
			name:      "Should down tenant/00003_create_group_table.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   3,
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
	}{
		{
			name:          "Should skip tenant/00001_copy_group_to_groups.go",
			target:        db.TenantTarget,
			version:       1,
			schemaVersion: ptr.PointTo(int64((0))),
		},
		{
			name:          "Should apply tenant/00001_copy_group_to_groups.go",
			target:        db.TenantTarget,
			version:       1,
			schemaVersion: ptr.PointTo(int64((3))),
			setupData: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()
				return func(db *multitenancy.DB) error {
					res := db.Exec(`INSERT INTO "group" (id, "name", description, "role", iam_identifier) VALUES(?, ?, ?, ?, ?)`,
						uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
					assert.NoError(t, res.Error)
					assert.GreaterOrEqual(t, int(res.RowsAffected), 1)
					return nil
				}
			},
			assertMigration: func(t *testing.T) func(db *multitenancy.DB) error {
				t.Helper()
				return func(db *multitenancy.DB) error {
					var count int
					err := db.Raw(`SELECT COUNT(*) FROM "groups"`).Scan(&count).Error
					assert.NoError(t, err)
					assert.GreaterOrEqual(t, count, 1)
					return nil
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := db.Migration{
				Type:   db.DataMigration,
				Target: tt.target,
			}

			setupVersion := tt.version - 1

			dbCon, m, tenant := setupDataMigration(t, DataMigrationSetup{
				Target:        tt.target,
				SchemaVersion: tt.schemaVersion,
				Version:       setupVersion,
			})

			if tt.setupData != nil {
				err := dbCon.WithTenant(t.Context(), tenant, tt.setupData(t))
				assert.NoError(t, err)
			}

			migrateVersion := tt.version

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
