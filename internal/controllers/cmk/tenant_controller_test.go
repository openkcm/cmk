package cmk_test

import (
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/testutils"
)

func startAPITenant(t *testing.T) (*multitenancy.DB, *http.ServeMux) {
	t.Helper()

	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		TenantCount:                  10,
		RequiresMultitenancyOrShared: true,
		Models:                       []driver.TenantTabler{&model.Tenant{}, &testutils.TestModel{}},
	})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})
}

func TestGetTenants(t *testing.T) {
	_, sv := startAPITenant(t)

	t.Run("Should 400 on list tenants with tenantID in path", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   "tenant-id",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should 200 on list tenants with sys path", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   "sys",
		})

		testutils.GetJSONBody[cmkapi.TenantList](t, w)
	})
}
