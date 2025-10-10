package cmk_test

import (
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
)

// startAPIServerTenantConfig starts the API server for keys and returns a pointer to the database
func startAPIServerTenantConfig(t *testing.T) (*multitenancy.DB, *http.ServeMux, string) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
	})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{}), tenants[0]
}

func TestAPIController_GetTenantKeystores(t *testing.T) {
	_, sv, tenant := startAPIServerTenantConfig(t)

	t.Run("Should 200 on get keystores", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants/keystores",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
