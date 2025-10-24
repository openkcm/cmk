package cmk_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// startAPIKeyConfigTags starts the API server and returns a db connection and a mux for testing
func startAPIKeyConfigTags(t *testing.T) (*multitenancy.DB, *http.ServeMux, string) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.KeyConfiguration{},
			&model.KeyConfigurationTag{},
		},
	})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{}), tenants[0]
}

// TestGetTagsForKeyConfiguration tests retrieving tags for a key configuration
func TestGetTagsForKeyConfiguration(t *testing.T) {
	db, sv, tenant := startAPIKeyConfigTags(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
		kc.Tags = []model.KeyConfigurationTag{
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag1",
				},
			},
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag2",
				},
			},
		}
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

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
				Method:   http.MethodGet,
				Endpoint: url,
				Tenant:   tenant,
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

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
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
			expectedTagCount:  2,
			expectedTagValues: []string{"tag1", "tag2"},
		},
		{
			name:              "InvalidRequestBody",
			keyConfigID:       keyConfig.ID.String(),
			requestBody:       "invalid-body",
			expectedStatus:    http.StatusBadRequest,
			expectedTagCount:  0,
			expectedTagValues: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPut,
				Endpoint: fmt.Sprintf("/keyConfigurations/%s/tags", tt.keyConfigID),
				Tenant:   tenant,
				Body:     testutils.WithJSON(t, tt.requestBody),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusNoContent {
				dbKeyConfig := model.KeyConfiguration{ID: keyConfig.ID}

				_, err := r.First(ctx, &dbKeyConfig, *repo.NewQuery().Preload(repo.Preload{"Tags"}))
				assert.NoError(t, err)

				assert.Len(t, dbKeyConfig.Tags, tt.expectedTagCount)
				tagValues := extractTagValues(dbKeyConfig.Tags)
				assert.ElementsMatch(t, tt.expectedTagValues, tagValues)
			}
		})
	}
}

// extractTagValues extracts tag values from a slice of model.Tag objects
func extractTagValues(tags []model.KeyConfigurationTag) []string {
	values := make([]string, len(tags))
	for i, tag := range tags {
		values[i] = tag.Value
	}

	return values
}
