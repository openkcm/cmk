package verbtampering_test

import (
	"errors"
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

var ErrForced = errors.New("forced")

func startAPIAndDB(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	dbConfig := testutils.TestDBConfig{}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig)

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
