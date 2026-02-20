package cmk_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkContext "github.com/openkcm/cmk/utils/context"
)

func startAPITenant(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux) {
	t.Helper()

	db, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithGenerateTenants(10))

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config: config.Config{Database: dbCfg},
	})
}

func TestGetTenants(t *testing.T) {
	db, sv := startAPITenant(t)
	r := sql.NewRepository(db)

	var tenants []model.Tenant

	err := r.List(t.Context(), model.Tenant{}, &tenants, *repo.NewQuery())
	assert.NoError(t, err)

	// Set issuerURL for first 3 tenants
	for i := range 3 {
		tenants[i].IssuerURL = "https://testissuer.example.com"
		_, err = r.Patch(t.Context(), &tenants[i], *repo.NewQuery())
		assert.NoError(t, err)

		tenantCtx := cmkContext.CreateTenantContext(t.Context(), tenants[i].ID)
		group := testutils.NewGroup(func(group *model.Group) {
			group.IAMIdentifier = "sysadmin"
		})

		err = r.Create(tenantCtx, group)
		assert.NoError(t, err)
	}

	t.Run("Should 200 on list tenants", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   tenants[0].ID,
			AdditionalContext: testutils.GetClientMap("test",
				[]string{"sysadmin", "othergroup"}),
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
			AdditionalContext: testutils.GetClientMap("test",
				[]string{"sysadmin", "othergroup"}),
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should 403 on list tenants without permission", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   tenants[0].ID,
			AdditionalContext: testutils.GetClientMap("test",
				[]string{"othergroup"}),
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestGetTenantInfo(t *testing.T) {
	db, sv := startAPITenant(t)
	r := sql.NewRepository(db)

	var tenant model.Tenant

	_, err := r.First(t.Context(), &tenant, *repo.NewQuery())
	assert.NoError(t, err)

	tenantCtx := cmkContext.CreateTenantContext(t.Context(), tenant.ID)

	authClient := testutils.NewAuthClient(tenantCtx, t, r, testutils.WithTenantAdminRole())

	group := testutils.NewGroup(func(group *model.Group) {
		group.IAMIdentifier = "sysadmin"
	})

	err = r.Create(tenantCtx, group)
	assert.NoError(t, err)

	t.Run("Should 403 on get tenant info that does not exist", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   "nonexistent-tenant-id",
			AdditionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should 403 on get tenant info without a user group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   "nonexistent-tenant-id",
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should 200 on get tenant by valid ID and client data", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
			AdditionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.Tenant](t, w)
		assert.Equal(t, tenant.ID, *resp.Id)
		assert.NotNil(t, resp.Role)
		expectedRole := strings.TrimPrefix(string(tenant.Role), "ROLE_")
		assert.Equal(t, cmkapi.TenantRole(expectedRole), *resp.Role)
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
			Method:            http.MethodGet,
			Endpoint:          "/tenantInfo",
			Tenant:            tenant.ID,
			AdditionalContext: authClient.GetClientMap(testutils.WithOverriddenGroup(1)),
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
