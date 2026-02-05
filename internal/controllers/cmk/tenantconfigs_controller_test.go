package cmk_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

// startAPIServerTenantConfig starts the API server for keys and returns a pointer to the database
func startAPIServerTenantConfig(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config: config.Config{Database: dbCfg},
	}), tenants[0]
}

func TestAPIController_GetTenantKeystores(t *testing.T) {
	db, sv, tenant := startAPIServerTenantConfig(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewAuthzKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should 200 on get keystores", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/tenantConfigurations/keystores",
			Tenant:            tenant,
			AdditionalContext: testutils.GetKeyAdminClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
