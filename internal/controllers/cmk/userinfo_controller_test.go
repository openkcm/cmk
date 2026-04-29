package cmk_test

import (
	"net/http"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func startAPIUserInfo(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string, *testutils.TestSigningKeyStorage) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})

	keyStorage := testutils.NewTestSigningKeyStorage(t)

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config:             config.Config{Database: dbCfg},
		EnableClientDataMW: true,
		SigningKeyStorage:  keyStorage,
	}), tenants[0], keyStorage
}

func TestGetUserInfo(t *testing.T) {
	db, sv, tenant, keyStorage := startAPIUserInfo(t)
	r := sql.NewRepository(db)

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	// Get private key for signing test requests
	privateKey, ok := keyStorage.GetPrivateKey(0)
	assert.True(t, ok, "test key should exist")

	t.Run("Should 200 on get user info with good client data", func(t *testing.T) {
		group := testutils.NewGroup(func(_ *model.Group) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		clientData := &auth.ClientData{
			Identifier: "user-123",
			Email:      "bob@example.com",
			GivenName:  "Bob",
			FamilyName: "Builder",
			Groups:     []string{group.IAMIdentifier, "some-other-group"},
		}
		headers := testutils.NewSignedClientDataHeadersFromStruct(t, clientData, privateKey, 0)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/userInfo",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.GetUserInfo200JSONResponse](t, w)

		assert.Equal(t, "user-123", resp.Identifier)
		assert.Equal(t, "bob@example.com", resp.Email)
		assert.Equal(t, "Bob", resp.GivenName)
		assert.Equal(t, "Builder", resp.FamilyName)
		assert.Contains(t, resp.Role, string(group.Role))
	})

	t.Run("Should 200 on get user info without group", func(t *testing.T) {
		clientData := &auth.ClientData{
			Identifier: "user-123",
			Email:      "bob@example.com",
			GivenName:  "Bob",
			FamilyName: "Builder",
			Groups:     []string{"some-other-group"},
		}
		headers := testutils.NewSignedClientDataHeadersFromStruct(t, clientData, privateKey, 0)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/userInfo",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.GetUserInfo200JSONResponse](t, w)

		assert.Equal(t, "user-123", resp.Identifier)
		assert.Equal(t, "bob@example.com", resp.Email)
		assert.Equal(t, "Bob", resp.GivenName)
		assert.Equal(t, "Builder", resp.FamilyName)
		assert.Contains(t, resp.Role, "")
	})

	t.Run("Should 500 on get user info with no client data", func(t *testing.T) {
		group := testutils.NewGroup(func(_ *model.Group) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/userInfo",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
