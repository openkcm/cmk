package benchmark_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func WithTotalKeys(ctx context.Context, r repo.Repo) func(*model.KeyConfiguration) error {
	return func(k *model.KeyConfiguration) error {
		var keys []*model.Key

		count, err := r.List(
			ctx,
			model.Key{},
			&keys,
			*repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(
						repo.KeyConfigIDField, k.ID))).SetLimit(1),
		)
		if err != nil {
			return err
		}

		k.TotalKeys = count

		return nil
	}
}

func WithTotalSystems(ctx context.Context, r repo.Repo) func(*model.KeyConfiguration) error {
	return func(k *model.KeyConfiguration) error {
		var sys []*model.System

		count, err := r.List(
			ctx,
			model.System{},
			&sys,
			*repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(
						repo.KeyConfigIDField, k.ID))).SetLimit(1),
		)
		if err != nil {
			return err
		}

		k.TotalSystems = count

		return nil
	}
}

func BenchmarkTotalKeyLoad(b *testing.B) {
	db, tenants, _ := testutils.NewTestDB(b, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.KeyConfiguration{}, &model.Key{}, &model.System{}},
	})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	N := 30
	for range N {
		keyConfigID := uuid.New()
		err := r.Create(ctx, testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.ID = keyConfigID
		}))
		assert.NoError(b, err)

		for range N {
			err = r.Create(ctx, testutils.NewKey(func(k *model.Key) {
				k.KeyConfigurationID = keyConfigID
			}))
			assert.NoError(b, err)

			err = r.Create(ctx, testutils.NewSystem(func(s *model.System) {
				s.KeyConfigurationID = ptr.PointTo(keyConfigID)
			}))
			assert.NoError(b, err)
		}
	}

	b.Run("LazyLoad fields on queries", func(b *testing.B) {
		for b.Loop() {
			var keyConfigs []*model.KeyConfiguration

			_, err := r.List(ctx, model.KeyConfiguration{}, &keyConfigs, *repo.NewQuery())
			assert.NoError(b, err)

			for _, k := range keyConfigs {
				res, err := repo.ToSharedModel(k, WithTotalKeys(ctx, r), WithTotalSystems(ctx, r))
				assert.NoError(b, err)
				assert.Equal(b, N, res.TotalSystems)
				assert.Equal(b, N, res.TotalKeys)
			}
		}
	})

	b.Run("Get totals with JOIN and COUNT", func(b *testing.B) {
		for b.Loop() {
			var keyConfigs []*model.KeyConfiguration

			_, err := r.List(
				ctx,
				model.KeyConfiguration{},
				&keyConfigs,
				*repo.NewQuery().Select(
					repo.NewSelectField(model.KeyConfiguration{}.TableName(), repo.QueryFunction{
						Function: repo.AllFunc,
					}),
					repo.NewSelectField(fmt.Sprintf("%s.%s", model.System{}.TableName(), repo.IDField), repo.QueryFunction{
						Function: repo.CountFunc,
						Distinct: true,
					}).SetAlias("total_systems"),
					repo.NewSelectField(fmt.Sprintf("%s.%s", model.Key{}.TableName(), repo.IDField), repo.QueryFunction{
						Function: repo.CountFunc,
						Distinct: true,
					}).SetAlias("total_keys"),
				).Join(repo.FullJoin, repo.JoinCondition{
					Table:     model.KeyConfiguration{},
					Field:     repo.IDField,
					JoinTable: model.Key{},
					JoinField: repo.KeyConfigIDField,
				}).Join(repo.FullJoin, repo.JoinCondition{
					Table:     model.KeyConfiguration{},
					Field:     repo.IDField,
					JoinTable: model.System{},
					JoinField: repo.KeyConfigIDField,
				}).GroupBy(fmt.Sprintf("%s.%s", model.KeyConfiguration{}.TableName(), repo.IDField)),
			)
			for _, k := range keyConfigs {
				assert.NoError(b, err)
				assert.Equal(b, N, k.TotalSystems)
				assert.Equal(b, N, k.TotalKeys)
			}
		}
	})
}
