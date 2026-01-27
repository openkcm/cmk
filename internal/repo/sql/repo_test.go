package sql_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestRepo_WithTenant(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	t.Run("Should run tenant action", func(t *testing.T) {
		err := r.WithTenant(ctx, testutils.TestModel{}, func(_ *multitenancy.DB) error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("Should run action without tenant on public table", func(t *testing.T) {
		err := r.WithTenant(t.Context(), &model.Tenant{}, func(_ *multitenancy.DB) error {
			return nil
		})
		assert.NoError(t, err)
	})
	t.Run("Should error on tenant specific repo wihtout tenant", func(t *testing.T) {
		err := r.WithTenant(t.Context(), model.Key{}, func(_ *multitenancy.DB) error {
			return nil
		})
		assert.ErrorIs(t, err, nethttp.ErrTenantInvalid)
	})
}

func TestRepo_List(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	n := 3
	for i := range n {
		err := r.Create(ctx, &testutils.TestModel{ID: uuid.New(), Name: fmt.Sprintf("test-%d", i)})
		assert.NoError(t, err)
	}

	t.Run("Should list resources", func(t *testing.T) {
		res := []*testutils.TestModel{}
		count, err := r.List(ctx, testutils.TestModel{}, &res, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, count, n)
		assert.Len(t, res, n)
	})

	t.Run("Should count total when paginated resources", func(t *testing.T) {
		res := []*testutils.TestModel{}
		limit := 1
		count, err := r.List(ctx, testutils.TestModel{}, &res, *repo.NewQuery().SetLimit(limit))
		assert.NoError(t, err)
		assert.Equal(t, count, n)
		assert.Len(t, res, limit)
	})

	t.Run("Should list IN", func(t *testing.T) {
		res := []*testutils.TestModel{}
		compositeKey := repo.NewCompositeKey().Where("name", []string{"test-0", "test-1"})
		query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))
		count, err := r.List(ctx, testutils.TestModel{}, &res, query)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestRepo_List_Order(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	n := 5
	for i := range n {
		err := r.Create(ctx, &testutils.TestModel{ID: uuid.New(), Name: strconv.Itoa(i)})
		assert.NoError(t, err)
	}

	t.Run("Should order resources descending", func(t *testing.T) {
		res := []*testutils.TestModel{}
		count, err := r.List(ctx, testutils.TestModel{}, &res, *repo.NewQuery().Order(repo.OrderField{
			Field:     "name",
			Direction: repo.Desc,
		}))
		assert.NoError(t, err)
		assert.Equal(t, n, count)
		assert.Len(t, res, n)
		assert.Equal(t, "4", res[0].Name)
		assert.Equal(t, "0", res[4].Name)
	})

	t.Run("Should order resources ascending", func(t *testing.T) {
		res := []*testutils.TestModel{}
		count, err := r.List(ctx, testutils.TestModel{}, &res, *repo.NewQuery().Order(repo.OrderField{
			Field:     "name",
			Direction: repo.Asc,
		}))
		assert.NoError(t, err)
		assert.Equal(t, n, count)
		assert.Len(t, res, n)
		assert.Equal(t, "0", res[0].Name)
		assert.Equal(t, "4", res[4].Name)
	})

	t.Run("Should get lowest name", func(t *testing.T) {
		var res *testutils.TestModel

		count, err := r.List(ctx, testutils.TestModel{}, &res, *repo.NewQuery().Order(repo.OrderField{
			Field:     "name",
			Direction: repo.Asc,
		}).SetLimit(1))
		assert.NoError(t, err)
		assert.Equal(t, 5, count)
		assert.Equal(t, "0", res.Name)
	})

	t.Run("Should get highest name", func(t *testing.T) {
		var res *testutils.TestModel

		count, err := r.List(ctx, testutils.TestModel{}, &res, *repo.NewQuery().Order(repo.OrderField{
			Field:     "name",
			Direction: repo.Desc,
		}).SetLimit(1))
		assert.NoError(t, err)
		assert.Equal(t, 5, count)
		assert.Equal(t, "4", res.Name)
	})
}

func TestRepo_Create(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	t.Run("Should create", func(t *testing.T) {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)

		res := &testutils.TestModel{ID: item.ID}

		_, err = r.First(ctx, res, *repo.NewQuery())
		assert.NoError(t, err)

		assert.Equal(t, item.Name, res.Name)
	})

	t.Run("Should error on duplicated key", func(t *testing.T) {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)

		err = r.Create(ctx, &item)
		assert.ErrorIs(t, err, repo.ErrUniqueConstraint)
	})
}

func TestRepo_First(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	testModel := &testutils.TestModel{
		ID:        uuid.New(),
		Name:      uuid.New().String(),
		CreatedAt: time.Now(),
	}

	err := r.Create(ctx, testModel)
	assert.NoError(t, err)

	t.Run("Should get resource", func(t *testing.T) {
		res := &testutils.TestModel{ID: testModel.ID}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Should error getting non existing resource", func(t *testing.T) {
		res := &testutils.TestModel{ID: uuid.New()}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.ErrorIs(t, err, repo.ErrNotFound)
		assert.False(t, ok)
	})

	t.Run("Should get resource by field", func(t *testing.T) {
		res := &testutils.TestModel{}
		ck := repo.NewCompositeKey().Where("name", testModel.ID)
		query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
		ok, err := r.First(ctx, res, query)
		assert.ErrorIs(t, err, repo.ErrNotFound)
		assert.False(t, ok)
	})

	t.Run("Should not get resource by field gt", func(t *testing.T) {
		res := &testutils.TestModel{}
		compositeKey := repo.NewCompositeKey().Where(
			"created_at", time.Now().AddDate(0, 0, 7), repo.Gt)
		query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))
		ok, err := r.First(ctx, res, query)
		assert.ErrorIs(t, err, repo.ErrNotFound)
		assert.False(t, ok)
	})

	t.Run("Should get resource by field lt", func(t *testing.T) {
		res := &testutils.TestModel{}
		compositeKey := repo.NewCompositeKey().Where(
			"created_at", time.Now().AddDate(0, 0, 7), repo.Lt)
		query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))
		ok, err := r.First(ctx, res, query)
		assert.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestRepo_Delete(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	t.Run("Should delete", func(t *testing.T) {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)
		ok, err := r.Delete(ctx, &item, *repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Should error delete non existing", func(t *testing.T) {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)
		ok, err := r.Delete(ctx, &testutils.TestModel{}, *repo.NewQuery())
		assert.ErrorIs(t, err, repo.ErrDeleteResource)
		assert.False(t, ok)
	})
}

func TestRepo_Patch(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	t.Run("Should patch", func(t *testing.T) {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)

		item.Name = "name-changed"
		_, err = r.Patch(ctx, &item, *repo.NewQuery())
		assert.NoError(t, err)

		res := &testutils.TestModel{ID: item.ID}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.Equal(t, item.Name, res.Name)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Should error on patch non existing resource", func(t *testing.T) {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)

		item = testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		ok, err := r.Patch(ctx, &item, *repo.NewQuery())
		assert.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestRepo_Set(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	t.Run("Should create if empty ", func(t *testing.T) {
		m := testutils.TestModel{ID: uuid.New(), Name: "test"}
		err := r.Set(ctx, &m)
		assert.NoError(t, err)

		res := &testutils.TestModel{ID: m.ID}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.Equal(t, m.Name, res.Name)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Should override if not empty ", func(t *testing.T) {
		m := testutils.TestModel{ID: uuid.New(), Name: "test-override"}
		err := r.Create(ctx, &m)
		assert.NoError(t, err)

		m.Name = "updated"
		err = r.Set(ctx, &m)
		assert.NoError(t, err)

		res := &testutils.TestModel{ID: m.ID}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.Equal(t, m.Name, res.Name)
		assert.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestRepo_Transaction(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	t.Run("Should rollback on error", func(t *testing.T) {
		m := testutils.TestModel{ID: uuid.New(), Name: uuid.NewString()}
		ctx, cancel := context.WithCancel(testutils.CreateCtxWithTenant(tenants[0]))

		_ = r.Transaction(ctx, func(ctx context.Context) error {
			err := r.Create(ctx, &m)
			assert.NoError(t, err)

			cancel()

			return nil
		})

		res := &testutils.TestModel{ID: m.ID}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("Should commit if no error", func(t *testing.T) {
		m := testutils.TestModel{ID: uuid.New(), Name: uuid.NewString()}
		ctx := testutils.CreateCtxWithTenant(tenants[0])
		_ = r.Transaction(ctx, func(ctx context.Context) error {
			err := r.Create(ctx, &m)
			assert.NoError(t, err)

			return nil
		})

		res := &testutils.TestModel{ID: m.ID}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, ok)
	})
}
