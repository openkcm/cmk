package dbmigration_test

import (
	"fmt"
	"testing"

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

func setupSharedMigration(t *testing.T, version int64) (*multitenancy.DB, db.Migrator) {
	t.Helper()

	dbCon, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		SharedVersion:  ptr.PointTo(version),
	}, testutils.WithGenerateTenants(0))

	r := sql.NewRepository(dbCon)
	m, err := db.NewMigrator(r, &config.Config{Database: dbCfg})
	assert.NoError(t, err)

	return dbCon, m
}

func setupTenantMigration(t *testing.T, version int64) (*multitenancy.DB, db.Migrator, string) {
	t.Helper()

	dbCon, tenant, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		TenantVersion:  ptr.PointTo(version),
	})

	r := sql.NewRepository(dbCon)
	m, err := db.NewMigrator(r, &config.Config{Database: dbCfg})
	assert.NoError(t, err)

	return dbCon, m, tenant[0]
}

func assertVersion(t *testing.T, dbCon *multitenancy.DB, version int64, tenant string) {
	t.Helper()

	var res GooseVersion
	query := fmt.Sprintf("SELECT version_id FROM %s ORDER BY id DESC", db.SchemaMigrationTable)

	err := dbCon.WithTenant(t.Context(), tenant, func(tx *multitenancy.DB) error {
		return tx.Raw(query).Scan(&res).Error
	})

	assert.NoError(t, err)
	assert.NoError(t, err)

	assert.Equal(t, version, res.Version)
}

func TestMissingScripts(t *testing.T) {
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
		assert.Subset(t, gormRows, gooseRows)
	})
}

func TestMigrations(t *testing.T) {
	tests := []struct {
		name      string
		target    db.MigrationTarget
		downgrade bool
		version   int64
	}{
		{
			name:      "Should up shared/0001_init_shared.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   1,
		},
		{
			name:      "Should down shared/0001_init_shared.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   1,
		},
		{
			name:      "Should up shared/0002_rename_keystore_pool_table.sql",
			downgrade: false,
			target:    db.SharedTarget,
			version:   2,
		},
		{
			name:      "Should down shared/0002_rename_keystore_pool_table.sql",
			downgrade: true,
			target:    db.SharedTarget,
			version:   2,
		},
		{
			name:      "Should up tenant/0001_init_shared.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   1,
		},
		{
			name:      "Should down tenant/0001_init_shared.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   1,
		},
		{
			name:      "Should up tenant/0002_create_tag_table.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   2,
		},
		{
			name:      "Should down tenant/0002_create_tag_table.sql",
			downgrade: true,
			target:    db.TenantTarget,
			version:   2,
		},
		{
			name:      "Should up tenant/0003_add_error_event_table.sql",
			downgrade: false,
			target:    db.TenantTarget,
			version:   3,
		},
		{
			name:      "Should down tenant/0003_add_error_event_table.sql",
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

			var dbCon *multitenancy.DB
			var m db.Migrator
			var tenant string
			if tt.target == db.SharedTarget {
				dbCon, m = setupSharedMigration(t, setupVersion)
				tenant = db.SharedSchema
			} else {
				dbCon, m, tenant = setupTenantMigration(t, setupVersion)
			}

			var migrateVersion int64
			if tt.downgrade {
				migrateVersion = tt.version - 1
			} else {
				migrateVersion = tt.version
			}

			err := m.MigrateTo(t.Context(), migration, migrateVersion)
			assert.NoError(t, err)

			assertVersion(t, dbCon, migrateVersion, tenant)
		})
	}
}
