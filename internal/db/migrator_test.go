package db_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

const (
	testTable = "test"
)

var (
	once         sync.Once
	psqlInstance config.Database
)

func createMigrationFiles(t *testing.T, table string) string {
	t.Helper()

	dir := t.TempDir()

	content := fmt.Sprintf(`
-- +goose Up 
	CREATE TABLE %s (id UUID PRIMARY KEY);
-- +goose Down
	DROP TABLE %s; 
	`, table, table)

	p := filepath.Join(dir, "0001_migration.sql")

	err := os.WriteFile(p, []byte(content), 0o600)
	assert.NoError(t, err)

	return dir
}

// Do not use testutils.NewDB as it migrates using Goose and this tests
// want to test versions
func setupMigrator(t *testing.T) (db.Migrator, string, *multitenancy.DB) {
	t.Helper()

	ctx := context.Background()

	once.Do(func() {
		dbCfg := testutils.TestDB
		testutils.StartPostgresSQL(t, &dbCfg, testcontainers.WithReuseByName(uuid.NewString()))
		psqlInstance = dbCfg
	})
	dbCfg := testutils.NewIsolatedDB(t, psqlInstance)
	dbCfg.Migrator.Shared.Schema = createMigrationFiles(t, testTable)
	dbCfg.Migrator.Tenant.Schema = createMigrationFiles(t, testTable)

	dbCon, err := db.StartDBConnection(ctx, dbCfg, []config.Database{})
	assert.NoError(t, err)

	err = dbCon.Migrator().CreateTable(&model.Tenant{})
	assert.NoError(t, err)

	tenant := testutils.NewTenant(func(_ *model.Tenant) {})
	testutils.CreateDBTenant(t, dbCon, tenant)

	m, err := db.NewMigrator(sql.NewRepository(dbCon), &config.Config{Database: dbCfg})
	assert.NoError(t, err)

	return m, tenant.ID, dbCon
}

func TestMigrator(t *testing.T) {
	tests := []struct {
		name      string
		migration db.Migration
	}{
		{
			name: "Should run only shared migrations",
			migration: db.Migration{
				Type:   db.SchemaMigration,
				Target: db.SharedTarget,
			},
		},
		{
			name: "Should run only tenant migrations",
			migration: db.Migration{
				Type:   db.SchemaMigration,
				Target: db.TenantTarget,
			},
		},
		{
			name: "Should run tenant and shared migrations",
			migration: db.Migration{
				Type:   db.SchemaMigration,
				Target: db.AllTarget,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, tenant, dbCon := setupMigrator(t)
			err := m.MigrateToLatest(t.Context(), tt.migration)
			assert.NoError(t, err)

			if tt.migration.Target == db.AllTarget || tt.migration.Target == db.SharedTarget {
				assert.True(t, dbCon.Migrator().HasTable(testTable))
			} else {
				assert.False(t, dbCon.Migrator().HasTable(testTable))
			}

			_ = dbCon.WithTenant(t.Context(), tenant, func(tx *multitenancy.DB) error {
				if tt.migration.Target == db.AllTarget || tt.migration.Target == db.TenantTarget {
					assert.True(t, tx.Migrator().HasTable(testTable))
				} else {
					assert.False(t, tx.Migrator().HasTable(testTable))
				}
				return nil
			})
		})
	}

	t.Run("Should not error on repeated migrations", func(t *testing.T) {
		m, _, dbCon := setupMigrator(t)
		err := m.MigrateToLatest(t.Context(), db.Migration{
			Type:   db.SchemaMigration,
			Target: db.SharedTarget,
		})
		assert.NoError(t, err)

		err = m.MigrateToLatest(t.Context(), db.Migration{
			Type:   db.SchemaMigration,
			Target: db.SharedTarget,
		})
		assert.NoError(t, err)

		assert.True(t, dbCon.Migrator().HasTable(testTable))
	})

	t.Run("Should error on rollback on empty databases", func(t *testing.T) {
		m, _, _ := setupMigrator(t)
		err := m.MigrateToLatest(t.Context(), db.Migration{
			Downgrade: true,
			Type:      db.SchemaMigration,
			Target:    db.SharedTarget,
		})
		assert.Error(t, err)
	})

	t.Run("Should rollback on DB containing migrations", func(t *testing.T) {
		m, _, dbCon := setupMigrator(t)
		err := m.MigrateToLatest(t.Context(), db.Migration{
			Type:   db.SchemaMigration,
			Target: db.SharedTarget,
		})
		assert.NoError(t, err)
		assert.True(t, dbCon.Migrator().HasTable(testTable))

		err = m.MigrateToLatest(t.Context(), db.Migration{
			Downgrade: true,
			Type:      db.SchemaMigration,
			Target:    db.SharedTarget,
		})
		assert.NoError(t, err)
		assert.False(t, dbCon.Migrator().HasTable(testTable))
	})

	t.Run("Should migrate to version", func(t *testing.T) {
		m, _, _ := setupMigrator(t)
		err := m.MigrateTo(t.Context(), db.Migration{
			Type:   db.SchemaMigration,
			Target: db.SharedTarget,
		}, 1)
		assert.NoError(t, err)
	})

	t.Run("Should error on unsupported migration", func(t *testing.T) {
		m, _, _ := setupMigrator(t)
		err := m.MigrateToLatest(t.Context(), db.Migration{
			Type:   "error",
			Target: "error",
		})
		assert.ErrorIs(t, err, db.ErrUnsupportedMigration)
	})

	t.Run("Should error if there are no migrations", func(t *testing.T) {
		m, _, _ := setupMigrator(t)
		err := m.MigrateToLatest(t.Context(), db.Migration{
			Type:   db.DataMigration,
			Target: db.AllTarget,
		})
		assert.Error(t, err)
	})
}
