package xss_test

import (
	"fmt"
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

const (
	apiGetKeyLabelsFmt         = "/key/%s/labels?$count=true"
	apiCreateOrUpdateLabelsFmt = "/key/%s/labels"
)

func startAPIAndDBForKeyLabels(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	dbConfig := testutils.TestDBConfig{}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig)

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

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	key := testutils.NewKey(func(k *model.Key) { k.KeyConfigurationID = keyConfig.ID })
	testutils.CreateTestEntities(ctx, t, r, key, keyConfig)

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodPost,
		Endpoint:          fmt.Sprintf(apiCreateOrUpdateLabelsFmt, key.ID.String()),
		Tenant:            tenant,
		Body:              testutils.WithJSON(t, inputLabels),
		AdditionalContext: authClient.GetClientMap(),
	})

	assert.Equal(t, http.StatusNoContent, w.Code)

	w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodGet,
		Endpoint:          fmt.Sprintf(apiGetKeyLabelsFmt, key.ID.String()),
		Tenant:            tenant,
		AdditionalContext: authClient.GetClientMap(),
	})

	assert.Equal(t, http.StatusOK, w.Code)
	response := testutils.GetJSONBody[cmkapi.LabelList](t, w)
	assert.Equal(t, output, response.Value)
}
