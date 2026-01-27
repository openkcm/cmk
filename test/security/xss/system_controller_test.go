package xss_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func startAPIAndDBForSystem(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	dbConfig := testutils.TestDBConfig{}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig)

	serverCfg := testutils.TestAPIServerConfig{
		Config: config.Config{
			ContextModels: config.ContextModels{
				System: config.System{
					OptionalProperties: map[string]config.SystemProperty{
						"test": {DisplayName: "a<SCRIPT></SCRIPT>b"},
					},
				},
			},
		},
	}
	sv := testutils.NewAPIServer(t, db, serverCfg)

	return db, sv, tenants[0]
}

func TestGetSystems_ForXSS(t *testing.T) {
	db, sv, tenant := startAPIAndDBForSystem(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	systemWithProps := testutils.NewSystem(func(s *model.System) {
		s.Properties = map[string]string{
			"test": "a<SCRIPT></SCRIPT>b",
		}
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		systemWithProps,
	)

	t.Run("Should show properties field on system with properties", func(t *testing.T) {
		expected := &map[string]any{"test": "ab"}
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/systems/%s", systemWithProps.ID),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.System](t, w)
		assert.Equal(t, expected, response.Properties)
	})
}
