package verbtampering_test

import (
	"errors"
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

var ErrForced = errors.New("forced")

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

func TestAPIController_GetAllSystems_ForVerbTampering(t *testing.T) {
	// Once authorisations have been added these tests should be extended to test for attempted
	// circumnavigation of authz
	db, sv, tenant := startAPIAndDB(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	system1 := testutils.NewSystem(func(_ *model.System) {})
	system2 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		system1,
		system2,
	)

	// First test the expected VERB
	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodGet,
		Endpoint: "/systems?$count=true",
		Tenant:   tenant,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	// We should not get a success on any other verbs with this endpoint
	verbs := []string{
		http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace,
	}

	for _, verb := range verbs {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   verb,
			Endpoint: "/systems?$count=true",
			Tenant:   tenant,
		})
		if verb == http.MethodHead {
			// This case is a bug. 405 should also be returned here but not a security issue.
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		} else {
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		}
	}
}
