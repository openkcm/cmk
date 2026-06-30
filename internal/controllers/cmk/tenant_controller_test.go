package cmk_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkContext "github.com/openkcm/cmk/utils/context"
)

func startAPITenant(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, *testutils.TestSigningKeyStorage) {
	t.Helper()

	db, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithGenerateTenants(10))

	keyStorage := testutils.NewTestSigningKeyStorage(t)
	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config:                   config.Config{Database: dbCfg},
		EnableBusinessUserDataMW: true,
		SigningKeyStorage:        keyStorage,
	}), keyStorage
}

func TestGetTenants(t *testing.T) {
	db, sv, keyStorage := startAPITenant(t)
	r := sql.NewRepository(db)

	var tenants []model.Tenant
	var headers http.Header

	err := r.List(t.Context(), model.Tenant{}, &tenants, *repo.NewQuery())
	assert.NoError(t, err)

	// Set issuerURL for first 3 tenants
	for i := range 3 {
		if i%2 == 0 {
			tenants[i].IssuerURL = "test"
			_, err = r.Patch(t.Context(), &tenants[i], *repo.NewQuery())
			assert.NoError(t, err)
		}

		tenantCtx := cmkContext.CreateTenantContext(t.Context(), tenants[i].ID)
		group := testutils.NewGroup(func(group *model.Group) {
			group.IAMIdentifier = "sysadmin"
		})

		err = r.Create(tenantCtx, group)
		assert.NoError(t, err)
	}

	clientData := &auth.ClientData{
		Identifier: "user-123",
		Email:      "bob@example.com",
		GivenName:  "Bob",
		FamilyName: "Builder",
		Groups:     []string{"sysadmin", "some-other-group"},
		AuthContext: map[string]string{
			"issuer": "test",
		},
	}
	// Get private key for signing test requests
	privateKey, ok := keyStorage.GetPrivateKey(0)
	assert.True(t, ok, "test key should exist")

	headers = testutils.NewSignedBusinessUserDataHeaders(t, clientData, privateKey, 0)
	assert.NotEmpty(t, headers)

	t.Run("Should fetch only tenants with issuer", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   tenants[0].ID,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.TenantList](t, w)
		assert.Len(t, resp.Value, 2)
	})

	t.Run("Should 403 on list tenants with non-existing tenant", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   "non-existing-tenant-id",
			Headers:  headers,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should 200 and be empty where there is no issuer", func(t *testing.T) {
		notAllowedClientData := &auth.ClientData{
			Identifier: "user-123",
			Email:      "bob@example.com",
			GivenName:  "Bob",
			FamilyName: "Builder",
			Groups:     []string{"test", "some-other-test-group"},
			AuthContext: map[string]string{
				"issuer": "non-existing",
			},
		}

		// Get private key for signing test requests
		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")

		headersNotAllowed := testutils.NewSignedBusinessUserDataHeaders(t, notAllowedClientData, privateKey, 0)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenants",
			Tenant:   tenants[0].ID,
			Headers:  headersNotAllowed,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.TenantList](t, w)
		assert.Empty(t, resp.Value)
	})
}

func TestGetTenantInfo(t *testing.T) {
	db, sv, keyStorage := startAPITenant(t)
	r := sql.NewRepository(db)

	var tenant model.Tenant
	var headers http.Header

	_, err := r.First(t.Context(), &tenant, *repo.NewQuery())
	assert.NoError(t, err)

	tenantCtx := cmkContext.CreateTenantContext(t.Context(), tenant.ID)

	tenant.IssuerURL = "https://testissuer.example.com"
	_, err = r.Patch(tenantCtx, &tenant, *repo.NewQuery())
	assert.NoError(t, err)

	group := testutils.NewGroup(func(group *model.Group) {
		group.IAMIdentifier = "sysadmin"
		group.Role = constants.TenantAdminRole
	})

	err = r.Create(tenantCtx, group)
	assert.NoError(t, err)

	clientData := &auth.ClientData{
		Identifier: "user-123",
		Email:      "bob@example.com",
		GivenName:  "Bob",
		FamilyName: "Builder",
		Groups:     []string{group.IAMIdentifier, "some-other-test-group"},
	}
	// Get private key for signing test requests
	privateKey, ok := keyStorage.GetPrivateKey(0)
	assert.True(t, ok, "test key should exist")
	headers = testutils.NewSignedBusinessUserDataHeaders(t, clientData, privateKey, 0)
	assert.NotEmpty(t, headers)

	clientDataNoGroups := &auth.ClientData{
		Identifier: "user-123",
		Email:      "bob@example.com",
		GivenName:  "Bob",
		FamilyName: "Builder",
		Groups:     []string{},
	}
	headersNoGroups := testutils.NewSignedBusinessUserDataHeaders(t, clientDataNoGroups, privateKey, 0)
	assert.NotEmpty(t, headersNoGroups)

	clientDataInvalidGroup := &auth.ClientData{
		Identifier: "user-123",
		Email:      "bob@example.com",
		GivenName:  "Bob",
		FamilyName: "Builder",
		Groups:     []string{"not-existing-group"},
	}
	headersInvalidGroup := testutils.NewSignedBusinessUserDataHeaders(t, clientDataInvalidGroup, privateKey, 0)
	assert.NotEmpty(t, headersInvalidGroup)

	t.Run("Should 403 on get tenant info that does not exist", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   "nonexistent-tenant-id",
			Headers:  headers,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should 200 on get tenant by valid ID and client data", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.Tenant](t, w)
		assert.NotNil(t, resp.Id)
		assert.Equal(t, tenant.ID, *resp.Id)
		assert.NotNil(t, resp.Role)
		expectedRole := strings.TrimPrefix(string(tenant.Role), "ROLE_")
		assert.Equal(t, cmkapi.TenantRole(expectedRole), *resp.Role)
		assert.Equal(t, tenant.Name, resp.Name)
	})

	t.Run("Should 403 on get tenant info without a user group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
			Headers:  headersNoGroups,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should 500 on get tenant by valid ID and no client data", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Should 403 on get tenant by valid ID and no valid group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
			Tenant:   tenant.ID,
			Headers:  headersInvalidGroup,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
