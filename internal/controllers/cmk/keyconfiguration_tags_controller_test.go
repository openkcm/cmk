package cmk_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
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
func startAPIKeyConfigTags(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string, *testutils.TestSigningKeyStorage) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	keyStorage := testutils.NewTestSigningKeyStorage(t)

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		EnableBusinessUserDataMW: true,
		SigningKeyStorage:        keyStorage,
	}), tenants[0], keyStorage
}

// TestGetTagsForKeyConfiguration tests retrieving tags for a key configuration
func TestGetTagsForKeyConfiguration(t *testing.T) {
	db, sv, tenant, keyStorage := startAPIKeyConfigTags(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	tags := []string{"tag1", "tag2"}

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(*model.KeyConfiguration) {},
		testutils.WithAuthBusinessUserDataKC(authClient))

	tag1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   keyConfig.ID,
		Key:          model.SystemTagKey,
		Value:        tags[0],
	}
	tag2 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   keyConfig.ID,
		Key:          model.SystemTagKey,
		Value:        tags[1],
	}
	testutils.CreateTestEntities(ctx, t, r, keyConfig, tag1, tag2)

	clientData := &auth.ClientData{
		Identifier: authClient.Identifier,
		Groups:     []string{authClient.Group.IAMIdentifier},
	}

	privateKey, ok := keyStorage.GetPrivateKey(0)
	assert.True(t, ok, "test key should exist")
	headers := testutils.NewSignedBusinessUserDataHeaders(t, clientData, privateKey, 0)

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
				Headers:  headers,
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
	db, sv, tenant, keyStorage := startAPIKeyConfigTags(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthBusinessUserDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	clientData := &auth.ClientData{
		Identifier: authClient.Identifier,
		Groups:     []string{authClient.Group.IAMIdentifier},
	}

	privateKey, ok := keyStorage.GetPrivateKey(0)
	assert.True(t, ok, "test key should exist")
	headers := testutils.NewSignedBusinessUserDataHeaders(t, clientData, privateKey, 0)

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
				Method:   http.MethodPut,
				Endpoint: fmt.Sprintf("/keyConfigurations/%s/tags", tt.keyConfigID),
				Tenant:   tenant,
				Body:     testutils.WithJSON(t, tt.requestBody),
				Headers:  headers,
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusNoContent {
				labels := []*model.ResourceLabel{}
				ck := repo.NewCompositeKey().
					Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
					Where(repo.ResourceIDField, keyConfig.ID).
					Where(repo.KeyField, model.SystemTagKey)
				err := r.List(ctx, &model.ResourceLabel{}, &labels, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
				assert.NoError(t, err)

				resTags := make([]string, 0, len(labels))
				for _, l := range labels {
					resTags = append(resTags, l.Value)
				}
				assert.ElementsMatch(t, tt.expectedTagValues, resTags)
			}
		})
	}
}
