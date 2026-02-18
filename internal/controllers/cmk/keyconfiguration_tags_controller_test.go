package cmk_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// startAPIKeyConfigTags starts the API server and returns a DB connection and a mux for testing
func startAPIKeyConfigTags(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{}), tenants[0]
}

// TestGetTagsForKeyConfiguration tests retrieving tags for a key configuration
func TestGetTagsForKeyConfiguration(t *testing.T) {
	db, sv, tenant := startAPIKeyConfigTags(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	tags := []string{"tag1", "tag2"}
	bytes, err := json.Marshal(tags)
	assert.NoError(t, err)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(*model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	tag := testutils.NewTag(func(t *model.Tag) {
		t.ID = keyConfig.ID
		t.Values = bytes
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, tag)

	tests := []struct {
		name              string
		keyConfigID       string
		count             bool
		expectedStatus    int
		expectedTagCount  int
		expectedTagValues []string
	}{
		{
			name:              "GetTagsSuccess",
			keyConfigID:       keyConfig.ID.String(),
			count:             false,
			expectedStatus:    http.StatusOK,
			expectedTagCount:  2,
			expectedTagValues: []string{"tag1", "tag2"},
		},
		{
			name:              "GetTagsSuccessWithCount",
			keyConfigID:       keyConfig.ID.String(),
			count:             true,
			expectedStatus:    http.StatusOK,
			expectedTagCount:  2,
			expectedTagValues: []string{"tag1", "tag2"},
		},
		{
			name:              "InvalidKeyConfigurationID",
			keyConfigID:       "invalid-id",
			count:             false,
			expectedStatus:    http.StatusBadRequest,
			expectedTagCount:  0,
			expectedTagValues: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/keyConfigurations/%s/tags", tt.keyConfigID)

			if tt.count {
				url += "?$count=true"
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          url,
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.TagList](t, w)
				assert.Len(t, response.Value, tt.expectedTagCount)
				assert.ElementsMatch(t, tt.expectedTagValues, response.Value)

				if tt.count {
					assert.NotNil(t, response.Count)
					assert.Equal(t, tt.expectedTagCount, *response.Count)
				}
			}
		})
	}
}

// TestAddTagsToKeyConfiguration tests adding tags to a key configuration
func TestAddTagsToKeyConfiguration(t *testing.T) {
	db, sv, tenant := startAPIKeyConfigTags(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	tests := []struct {
		name              string
		keyConfigID       string
		requestBody       any
		expectedStatus    int
		expectedTagCount  int
		expectedTagValues []string
	}{
		{
			name:              "AddTagsSuccess",
			keyConfigID:       keyConfig.ID.String(),
			requestBody:       cmkapi.Tags{Tags: []string{"tag1", "tag2"}},
			expectedStatus:    http.StatusNoContent,
			expectedTagValues: []string{"tag1", "tag2"},
		},
		{
			name:              "InvalidRequestBody",
			keyConfigID:       keyConfig.ID.String(),
			requestBody:       "invalid-body",
			expectedStatus:    http.StatusBadRequest,
			expectedTagValues: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPut,
				Endpoint:          fmt.Sprintf("/keyConfigurations/%s/tags", tt.keyConfigID),
				Tenant:            tenant,
				Body:              testutils.WithJSON(t, tt.requestBody),
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusNoContent {
				tag := &model.Tag{ID: keyConfig.ID}

				_, err := r.First(ctx, tag, *repo.NewQuery())
				assert.NoError(t, err)

				resTags := []string{}
				err = json.Unmarshal(tag.Values, &resTags)
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedTagValues, resTags)
			}
		})
	}
}
