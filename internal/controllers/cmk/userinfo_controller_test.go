package cmk_test

import (
	"net/http"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func startAPIUserInfo(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config: config.Config{Database: dbCfg},
	}), tenants[0]
}

func TestGetUserInfo(t *testing.T) {
	db, sv, tenant := startAPIUserInfo(t)
	r := sql.NewRepository(db)

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	t.Run("Should 200 on get user info with good client data", func(t *testing.T) {
		group := testutils.NewGroup(func(_ *model.Group) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/userInfo",
			Tenant:   tenant,
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: "user-123",
					Email:      "bob@example.com",
					GivenName:  "Bob",
					FamilyName: "Builder",
					Groups:     []string{group.IAMIdentifier, "some-other-group"},
				},
			},
		})

		assert.Equal(t, http.StatusOK, w.Code)
		resp := testutils.GetJSONBody[cmkapi.GetUserInfo200JSONResponse](t, w)

		assert.Equal(t, "user-123", resp.Identifier)
		assert.Equal(t, "bob@example.com", resp.Email)
		assert.Equal(t, "Bob", resp.GivenName)
		assert.Equal(t, "Builder", resp.FamilyName)
		assert.Contains(t, resp.Role, string(group.Role))
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
