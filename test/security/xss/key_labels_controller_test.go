package xss_test

import (
	"fmt"
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

const (
	apiGetKeyLabelsFmt         = "/key/%s/labels?$count=true"
	apiCreateOrUpdateLabelsFmt = "/key/%s/labels"
)

func startAPIAndDBForKeyLabels(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	cfg := &config.Config{
		Database: integrationutils.DB,
	}
	testutils.StartPostgresSQL(t, &cfg.Database)

	dbConfig := testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Key{},
			&model.KeyLabel{},
		},
	}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig,
		testutils.WithDatabase(cfg.Database),
	)

	sv := testutils.NewAPIServer(t, db,
		testutils.TestAPIServerConfig{})

	return db, sv, tenants[0]
}

func TestLabelsController_Labels_ForXSS(t *testing.T) {
	inputLabels := []cmkapi.Label{{
		Key:   "Hello <STYLE></STYLE>World",
		Value: ptr.PointTo("Hello <STYLE></STYLE>World"),
	}}
	output := []cmkapi.Label{{
		Key:   "Hello World",
		Value: ptr.PointTo("Hello World"),
	}}

	db, sv, tenant := startAPIAndDBForKeyLabels(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	key := testutils.NewKey(func(_ *model.Key) {})
	testutils.CreateTestEntities(ctx, t, r, key)

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodPost,
		Endpoint: fmt.Sprintf(apiCreateOrUpdateLabelsFmt, key.ID.String()),
		Tenant:   tenant,
		Body:     testutils.WithJSON(t, inputLabels),
	})

	assert.Equal(t, http.StatusNoContent, w.Code)

	w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodGet,
		Endpoint: fmt.Sprintf(apiGetKeyLabelsFmt, key.ID.String()),
		Tenant:   tenant,
	})

	assert.Equal(t, http.StatusOK, w.Code)
	response := testutils.GetJSONBody[cmkapi.LabelList](t, w)
	assert.Equal(t, output, response.Value)
}
