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
)

func startAPIAndDBForKeyConfigTags(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	dbConfig := testutils.TestDBConfig{}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig)

	sv := testutils.NewAPIServer(t, db,
		testutils.TestAPIServerConfig{})

	return db, sv, tenants[0]
}

func TestAddTagsToKeyConfiguration_ForXSS(t *testing.T) {
	db, sv, tenant := startAPIAndDBForKeyConfigTags(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	inputTags := []string{"tag1", "Hello <STYLE></STYLE>World"}
	outputTags := []string{"tag1", "Hello World"}

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodPut,
		Endpoint:          fmt.Sprintf("/keyConfigurations/%s/tags", keyConfig.ID.String()),
		Tenant:            tenant,
		Body:              testutils.WithJSON(t, cmkapi.Tags{Tags: inputTags}),
		AdditionalContext: authClient.GetClientMap(),
	})
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodGet,
		Endpoint:          fmt.Sprintf("/keyConfigurations/%s/tags", keyConfig.ID.String()),
		Tenant:            tenant,
		AdditionalContext: authClient.GetClientMap(),
	})
	assert.Equal(t, http.StatusOK, w.Code)

	if w.Code == http.StatusOK {
		response := testutils.GetJSONBody[cmkapi.TagList](t, w)
		assert.Len(t, response.Value, 2)
		assert.ElementsMatch(t, outputTags, response.Value)
	}
}
