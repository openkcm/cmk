package repo_test

import (
	"sync"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
)

func TestConcurrency(t *testing.T) {
	tenantCount := 10
	itemPerTenant := 10
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&testutils.TestModel{}},
	}, testutils.WithGenerateTenants(tenantCount))
	r := sql.NewRepository(db)

	wg := sync.WaitGroup{}
	wg.Add(itemPerTenant * len(tenants))

	itemsByTenant := map[string][]*testutils.TestModel{}

	// create uniques items for every tenant
	for _, tenant := range tenants {
		for range itemPerTenant {
			itemsByTenant[tenant] = append(itemsByTenant[tenant], &testutils.TestModel{
				ID:   uuid.New(),
				Name: uuid.NewString(),
			})
		}
	}

	// Add items for the tenants in concurrency
	for i := range itemPerTenant {
		for _, tenant := range tenants {
			go func() {
				defer wg.Done()

				err := r.Create(testutils.CreateCtxWithTenant(tenant), itemsByTenant[tenant][i])
				assert.NoError(t, err)
			}()
		}
	}

	wg.Wait()

	for i, tenant := range tenants {
		otherTenant := tenants[(i+1)%itemPerTenant]

		for j := range itemPerTenant {
			item := itemsByTenant[tenant][j]

			// Existing item
			res := testutils.TestModel{ID: item.ID}
			ok, err := r.First(testutils.CreateCtxWithTenant(tenant), &res, *repo.NewQuery())
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, item.Name, res.Name)
			assert.Equal(t, item.ID, res.ID)

			// Non-Existing item
			ok, err = r.First(testutils.CreateCtxWithTenant(otherTenant), &res, *repo.NewQuery())
			assert.False(t, ok)
			assert.Error(t, err)
		}
	}
}

func TestProcessInBatch(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&testutils.TestModel{}},
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	for range 7 {
		item := &testutils.TestModel{
			ID:   uuid.New(),
			Name: "batch_item_" + uuid.NewString(),
		}
		err := r.Create(ctx, item)
		assert.NoError(t, err)
	}

	t.Run("should process all items in single batch when total count is less than batch size", func(t *testing.T) {
		baseQuery := repo.NewQuery()
		batchSize := 10

		processedItems := []*testutils.TestModel{}
		processFunc := func(items []*testutils.TestModel) error {
			processedItems = append(processedItems, items...)
			return nil
		}

		// Act
		err := repo.ProcessInBatch[testutils.TestModel](ctx, r, baseQuery, batchSize, processFunc)

		// Verify
		assert.NoError(t, err)
		assert.Len(t, processedItems, 7)
	})

	t.Run("should process all items in multiple batches", func(t *testing.T) {
		baseQuery := repo.NewQuery()
		batchSize := 3

		processedItems := []*testutils.TestModel{}
		batchCount := 0
		processFunc := func(items []*testutils.TestModel) error {
			batchCount++

			processedItems = append(processedItems, items...)
			// Verify batch sizes 7 total 3 + 3 + 1
			if batchCount <= 2 {
				assert.Len(t, items, 3, "First two batches should have 3 items each")
			} else {
				assert.Len(t, items, 1, "Last batch should have 1 item")
			}

			return nil
		}

		// Act
		err := repo.ProcessInBatch[testutils.TestModel](ctx, r, baseQuery, batchSize, processFunc)

		// Verify
		assert.NoError(t, err)
		assert.Len(t, processedItems, 7)
		assert.Equal(t, 3, batchCount, "Should process in 3 batches")
	})

	t.Run("should handle empty result set", func(t *testing.T) {
		baseQuery := repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("name", "fake_value"),
			),
		)
		batchSize := 10

		processCallCount := 0
		processFunc := func(items []*testutils.TestModel) error {
			processCallCount++

			assert.Empty(t, items)

			return nil
		}

		// Act
		err := repo.ProcessInBatch[testutils.TestModel](ctx, r, baseQuery, batchSize, processFunc)

		// Verify
		assert.NoError(t, err)
		assert.Equal(t, 1, processCallCount, "check proccessFunc is called on no data")
	})
}

func TestProcessInBatchWithOptionsDeleteMode(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&testutils.TestModel{}},
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	t.Run("should delete all items without skipping when DeleteMode is enabled", func(t *testing.T) {
		total := 7
		for range total {
			item := &testutils.TestModel{
				ID:   uuid.New(),
				Name: "delete_test_" + uuid.NewString(),
			}
			err := r.Create(ctx, item)
			assert.NoError(t, err)
		}

		baseQuery := repo.NewQuery()
		batchSize := 3

		processedItems := []*testutils.TestModel{}
		batchCount := 0
		processFunc := func(items []*testutils.TestModel) error {
			batchCount++
			processedItems = append(processedItems, items...)
			for _, item := range items {
				_, err := r.Delete(ctx, item, *repo.NewQuery())
				if err != nil {
					return err
				}
			}
			return nil
		}

		// Act with DeleteMode enabled
		err := repo.ProcessInBatchWithOptions[testutils.TestModel](
			ctx,
			r,
			baseQuery,
			batchSize,
			repo.BatchProcessOptions{DeleteMode: true},
			processFunc,
		)

		// Verify
		assert.NoError(t, err)
		assert.Len(t, processedItems, total, "Should process all items")
		assert.Equal(t, 4, batchCount, "Should process in 4 batches (3+3+1+0) - delete mode"+
			" always processes one extra iteration to detect there are no more items.")

		// Verify all items were actually deleted
		var remainingItems []*testutils.TestModel
		count, err := r.List(ctx, testutils.TestModel{}, &remainingItems, *baseQuery)
		assert.NoError(t, err)
		assert.Equal(t, 0, count, "All items should be deleted")
	})

	t.Run("should handle empty result set with DeleteMode enabled", func(t *testing.T) {
		baseQuery := repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("name", "nonexistent_value"),
			),
		)
		batchSize := 10

		processCallCount := 0
		processFunc := func(items []*testutils.TestModel) error {
			processCallCount++
			assert.Empty(t, items)
			return nil
		}

		// Act
		err := repo.ProcessInBatchWithOptions[testutils.TestModel](
			ctx,
			r,
			baseQuery,
			batchSize,
			repo.BatchProcessOptions{DeleteMode: true},
			processFunc,
		)

		// Verify
		assert.NoError(t, err)
		assert.Equal(t, 1, processCallCount, "processFunc should be called once even with no data")
	})
}

func TestGetTenant(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&testutils.TestModel{}},
	})
	r := sql.NewRepository(db)

	t.Run("should return tenant when found", func(t *testing.T) {
		// Arrange
		tenantID := tenants[0]
		ctx := testutils.CreateCtxWithTenant(tenantID)

		// Act
		tenant, err := repo.GetTenant(ctx, r)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, tenant)
		assert.Equal(t, tenantID, tenant.ID)
	})

	t.Run("should return ErrTenantNotFound when tenant does not exist", func(t *testing.T) {
		// Arrange
		nonExistentTenantID := uuid.NewString()
		ctx := testutils.CreateCtxWithTenant(nonExistentTenantID)

		// Act
		tenant, err := repo.GetTenant(ctx, r)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, tenant)
		assert.ErrorIs(t, err, repo.ErrTenantNotFound)
	})

	t.Run("should return error when no tenant in context", func(t *testing.T) {
		// Arrange
		ctx := testutils.CreateCtxWithTenant("")

		// Act
		tenant, err := repo.GetTenant(ctx, r)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, tenant)
	})
}
