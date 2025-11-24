package cmk_test

import (
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkContext "github.com/openkcm/cmk/utils/context"
)

func startAPITenant(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux) {
	t.Helper()

	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&model.Group{}, &model.KeyConfiguration{}},
	}, testutils.WithGenerateTenants(10))

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})
}

func TestGetTenants(t *testing.T) {
	db, sv := startAPITenant(t)
	r := sql.NewRepository(db)

	var tenants []model.Tenant

	_, err := r.List(t.Context(), model.Tenant{}, &tenants, *repo.NewQuery())
	assert.NoError(t, err)

	// Set issuerURL for first 3 tenants
	for i := range 3 {
		tenants[i].IssuerURL = "https://testissuer.example.com"
		_, err = r.Patch(t.Context(), &tenants[i], *repo.NewQuery())
		assert.NoError(t, err)
	}

	t.Run("Should 200 on list tenants", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   tenants[0].ID,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.TenantList](t, w)
		assert.Len(t, resp.Value, 3)
	})

	t.Run("Should 404 on list tenants with non-existing tenant", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   "non-existing-tenant-id",
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetTenantInfo(t *testing.T) {
	db, sv := startAPITenant(t)
	r := sql.NewRepository(db)

	var tenant model.Tenant

	_, err := r.First(t.Context(), &tenant, *repo.NewQuery())
	assert.NoError(t, err)

	tenantCtx := cmkContext.CreateTenantContext(t.Context(), tenant.ID)
	group := testutils.NewGroup(func(group *model.Group) {
		group.IAMIdentifier = "sysadmin"
	})

	err = r.Create(tenantCtx, group)
	assert.NoError(t, err)

	t.Run("Should 404 on get tenant info that does not exist", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   "nonexistent-tenant-id",
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should 200 on get tenant by valid ID and client data", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups: []string{"sysadmin", "othergroup"},
				},
			},
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.Tenant](t, w)
		assert.Equal(t, tenant.ID, *resp.Id)
	})

	t.Run("Should 403 on get tenant by valid ID and no client data", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should 403 on get tenant by valid ID and no valid group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
			AdditionalContext: map[any]any{
				"Groups": []string{"otheradm"},
			},
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
