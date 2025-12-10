package benchmark_test

import (
	"sync"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm/logger"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
)

func setupBenchmark(b *testing.B, nTenants int) *multitenancy.DB {
	b.Helper()

	dbCfg := config.Database{}
	testutils.StartPostgresSQL(b, &dbCfg, testcontainers.WithReuseByName(uuid.NewString()))

	models := []driver.TenantTabler{
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
		&model.ImportParams{},
		&model.KeystoreConfiguration{},
		&model.Event{},
	}
	// Disable tenant creation
	db, _, _ := testutils.NewTestDB(b, testutils.TestDBConfig{
		Models: models,
		Logger: logger.Default.LogMode(logger.Error),
	}, testutils.WithGenerateTenants(0))

	err := db.RegisterModels(b.Context(), models...)
	require.NoError(b, err)
	err = db.MigrateSharedModels(b.Context())
	require.NoError(b, err)
	for range nTenants {
		err := db.Create(testutils.NewTenant(func(_ *model.Tenant) {})).Error
		require.NoError(b, err)
	}

	return db
}

func BenchmarkMigration(b *testing.B) {
	nTenants := 500
	paginationLimit := 50
	b.Run("Migrate Sequentially", func(b *testing.B) {
		db := setupBenchmark(b, nTenants)
		r := sql.NewRepository(db)
		ctx := b.Context()

		for b.Loop() {
			err := repo.ProcessInBatch(ctx, r, repo.NewQuery(), paginationLimit, func(tenants []*model.Tenant) error {
				for _, tenant := range tenants {
					ctx := log.InjectTenant(ctx, tenant)

					err := db.MigrateTenantModels(ctx, tenant.SchemaName)
					if err != nil {
						log.Error(ctx, "Failed to migrate tenant", err)
					}
				}

				return nil
			})
			require.NoError(b, err)
		}
	})

	b.Run("Migrate in parallel pooled", func(b *testing.B) {
		db := setupBenchmark(b, nTenants)
		r := sql.NewRepository(db)
		ctx := b.Context()

		for b.Loop() {
			wg := sync.WaitGroup{}
			sem := semaphore.NewWeighted(8)

			err := repo.ProcessInBatch(ctx, r, repo.NewQuery(), paginationLimit, func(tenants []*model.Tenant) error {
				for _, tenant := range tenants {
					wg.Add(1)
					go func(tenant *model.Tenant) {
						defer wg.Done()
						err := sem.Acquire(ctx, 1)
						if err != nil {
							log.Error(ctx, "Failed to start tenant migration", err)
							return
						}
						defer sem.Release(1)

						ctx := log.InjectTenant(ctx, tenant)

						err = db.MigrateTenantModels(ctx, tenant.SchemaName)
						if err != nil {
							log.Error(ctx, "Failed to migrate tenant", err)
						}
					}(tenant)
				}

				return nil
			})
			wg.Wait()
			require.NoError(b, err)
		}
	})

	b.Run("Migrate in parallel", func(b *testing.B) {
		db := setupBenchmark(b, nTenants)
		r := sql.NewRepository(db)
		ctx := b.Context()

		for b.Loop() {
			wg := sync.WaitGroup{}
			err := repo.ProcessInBatch(ctx, r, repo.NewQuery(), paginationLimit, func(tenants []*model.Tenant) error {
				for _, tenant := range tenants {
					wg.Add(1)
					go func(tenant *model.Tenant) {
						defer wg.Done()
						ctx := log.InjectTenant(ctx, tenant)

						err := db.MigrateTenantModels(ctx, tenant.SchemaName)
						if err != nil {
							log.Error(ctx, "Failed to migrate tenant", err)
						}
					}(tenant)
				}

				return nil
			})
			wg.Wait()
			require.NoError(b, err)
		}
	})
}
