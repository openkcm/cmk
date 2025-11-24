package parampollution_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var ErrForced = errors.New("forced")

func startAPIAndDB(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	cfg := &config.Config{
		Database: integrationutils.DB,
	}
	integrationutils.StartPostgresSQL(t, &cfg.Database)

	dbConfig := testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.System{},
			&model.SystemProperty{},
			&model.KeyConfiguration{},
			&model.Key{},
			&model.KeyVersion{},
			&model.KeyLabel{},
		}}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig,
		testutils.WithDatabase(cfg.Database),
	)

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	return db, sv, tenants[0]
}

func TestAPIController_GetAllSystems_ForParameterPollution(t *testing.T) {
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

	// First test ok with single parameter
	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodGet,
		Endpoint: "/systems?$count=true",
		Tenant:   tenant,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	// Vulnerability is when same parameter passed twice. Validation is applied only
	// to one parameter and other is the one which is processed
	w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodGet,
		Endpoint: "/systems?$count=true&$count=true",
		Tenant:   tenant,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
