package sqlinjection_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func startAPIAndDB(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	dbConfig := testutils.TestDBConfig{}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig)

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	return db, sv, tenants[0]
}

func TestAPIController_GetAllSystems_ForInjection(t *testing.T) {
	// Once authorisations have been added these tests should be extended to test for attempted
	// circumnavigation of authz
	db, sv, tenant := startAPIAndDB(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	system1 := testutils.NewSystem(func(_ *model.System) {})
	system2 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		s.Status = cmkapi.SystemStatusPROCESSING
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		system1,
		system2,
	)

	t.Run("Test normal paths", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/systems?$count=true",
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)
		response := testutils.GetJSONBody[cmkapi.SystemList](t, w)
		assert.Equal(t, 2, *response.Count)

		w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/systems?$count=true&$filter=status eq 'DISCONNECTED'",
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)
		response = testutils.GetJSONBody[cmkapi.SystemList](t, w)
		assert.Equal(t, 1, *response.Count)
	})

	t.Run("Test attempts to get all table contents", func(t *testing.T) {
		attackStrings := []string{
			"status eq '' OR 1=1",
			"status eq 'OR 1=1'",
			"status eq OR 1=1",
		}

		for _, attackString := range attackStrings {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/systems?$count=true&$filter=" + attackString,
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})

			response := testutils.GetJSONBody[cmkapi.SystemList](t, w)
			if w.Code == http.StatusOK {
				assert.Equal(t, 0, *response.Count)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.Nil(t, response.Count)
			}
		}
	})

	t.Run("Test attempts to drop tables", func(t *testing.T) {
		attackStrings := []string{
			"');drop table systems;",
			"');drop table \"systems\";",
			"');drop table 'systems';",

			"'');drop table systems;",
			"'');drop table \"systems\";",
			"'');drop table 'systems';",

			"drop table systems;",
			"drop table \"systems\";",
			"drop table 'systems';",
		}

		for _, attackString := range attackStrings {
			testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/systems?$count=true&$filter=" + attackString,
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})

			// Check there are still entries in the table
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/systems?$count=true",
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, http.StatusOK, w.Code)
			response := testutils.GetJSONBody[cmkapi.SystemList](t, w)
			assert.Equal(t, 2, *response.Count)
		}
	})
}
