package model_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
)

func TestTenantsTable(t *testing.T) {
	t.Run("Should have table name tenants", func(t *testing.T) {
		expectedTableName := "public.tenants"

		tableName := model.Tenant{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a public table", func(t *testing.T) {
		assert.True(t, model.Tenant{}.IsSharedModel())
	})

	t.Run("Should have unique combination id and region", func(t *testing.T) {
		db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models:                       []driver.TenantTabler{&model.Tenant{}, &testutils.TestModel{}},
			RequiresMultitenancyOrShared: true,
		})

		r := sql.NewRepository(db)
		err := r.Create(t.Context(), &model.Tenant{
			ID:     "test-id",
			Region: "test-region",
			TenantModel: multitenancy.TenantModel{
				DomainURL:  "test-domain.example.com",
				SchemaName: "test-schema",
			},
		})
		assert.NoError(t, err)

		err = r.Create(t.Context(), &model.Tenant{
			ID:     "test-id1",
			Region: "test-region",
			TenantModel: multitenancy.TenantModel{
				DomainURL:  "test-domain1.example.com",
				SchemaName: "test-schema1",
			},
		})
		assert.NoError(t, err)
	})
}
