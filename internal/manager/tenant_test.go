package manager_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
)

func TestTenantManager(t *testing.T) {
	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		TenantCount:                  10,
		RequiresMultitenancyOrShared: true,
		Models:                       []driver.TenantTabler{&testutils.TestModel{}},
	})
	repo := sql.NewRepository(db)
	m := manager.NewTenantManager(repo)

	t.Run("Should get tenant info", func(t *testing.T) {
		tenant := tenants[5]
		tenantModel, err := m.GetTenant(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Equal(t, tenant, tenantModel.ID)
	})
	t.Run("Should list tenants", func(t *testing.T) {
		tenantsModel, _, err := m.ListTenantInfo(t.Context(), 0, 0)
		assert.NoError(t, err)

		for i := range tenantsModel {
			assert.Equal(t, tenants[i], tenantsModel[i].ID)
		}
	})
}
