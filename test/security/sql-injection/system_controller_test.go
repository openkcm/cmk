package sqlinjection_test

import (
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	integrationutils "github.tools.sap/kms/cmk/test/integration/integration_utils"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
	"github.tools.sap/kms/cmk/utils/ptr"
)

func startAPIAndDB(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	cfg := &config.Config{
		Database: integrationutils.DB,
	}
	testutils.StartPostgresSQL(t, &cfg.Database)

	dbConfig := testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.System{},
			&model.SystemProperty{},
			&model.KeyConfiguration{},
			&model.Key{},
			&model.KeyVersion{},
			&model.KeyLabel{},
		},
	}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig,
		testutils.WithDatabase(cfg.Database),
	)

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	return db, sv, tenants[0]
}

func TestAPIController_GetAllSystems_ForInjection(t *testing.T) {
	// Once authorisations have been added these tests should be extended to test for attempted
	// circumnavigation of authz
	db, sv, tenant := startAPIAndDB(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
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
			Method:   http.MethodGet,
			Endpoint: "/systems?$count=true",
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		response := testutils.GetJSONBody[cmkapi.SystemList](t, w)
		assert.Equal(t, 2, *response.Count)

		w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systems?$count=true&$filter=status eq 'DISCONNECTED'",
			Tenant:   tenant,
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
				Method:   http.MethodGet,
				Endpoint: "/systems?$count=true&$filter=" + attackString,
				Tenant:   tenant,
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
				Method:   http.MethodGet,
				Endpoint: "/systems?$count=true&$filter=" + attackString,
				Tenant:   tenant,
			})

			// Check there are still entries in the table
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/systems?$count=true",
				Tenant:   tenant,
			})
			assert.Equal(t, http.StatusOK, w.Code)
			response := testutils.GetJSONBody[cmkapi.SystemList](t, w)
			assert.Equal(t, 2, *response.Count)
		}
	})
}
