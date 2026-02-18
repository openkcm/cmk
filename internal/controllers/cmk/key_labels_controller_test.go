package cmk_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	apiGetKeyLabelsFmt         = "/key/%s/labels?$count=true"
	apiCreateOrUpdateLabelsFmt = "/key/%s/labels"
)

var regularLabels = []cmkapi.Label{
	{
		Key:   "foo",
		Value: ptr.PointTo("bar"),
	},
	{
		Key:   "region/az",
		Value: ptr.PointTo("eu-west-1/a"),
	},
}

var labelWithEmptyValue = []cmkapi.Label{
	{
		Key:   "foo",
		Value: ptr.PointTo(""),
	},
}

var errorLabel = []cmkapi.Label{
	{
		Value: ptr.PointTo(""),
	},
}

// startAPIKeyLabels starts the API server for key labels and returns a pointer to the database
func startAPIKeyLabels(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{}), tenants[0]
}

func TestLabelsController_GetKeyLabels(t *testing.T) {
	t.Run("Should get existing labels", func(t *testing.T) {
		db, sv, tenant := startAPIKeyLabels(t)
		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
			testutils.WithAuthClientDataKC(authClient))

		expected := []cmkapi.Label{
			{
				Key:   "foo",
				Value: ptr.PointTo("bar"),
			},
			{
				Key:   "region/az",
				Value: ptr.PointTo("eu-west-1/a"),
			},
		}
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.KeyLabels = []model.KeyLabel{
				*testutils.NewKeyLabel(func(l *model.KeyLabel) {
					l.Key = "foo"
					l.Value = "bar"
					l.ResourceID = keyID
				}),
				*testutils.NewKeyLabel(func(l *model.KeyLabel) {
					l.Key = "region/az"
					l.Value = "eu-west-1/a"
					l.ResourceID = keyID
				}),
			}
		})

		testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          fmt.Sprintf(apiGetKeyLabelsFmt, key.ID.String()),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)
		response := testutils.GetJSONBody[cmkapi.LabelList](t, w)
		assert.Equal(t, expected, response.Value)
	})

	t.Run("Should get no labels on empty key", func(t *testing.T) {
		db, sv, tenant := startAPIKeyLabels(t)
		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
			testutils.WithAuthClientDataKC(authClient))

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          fmt.Sprintf(apiGetKeyLabelsFmt, key.ID.String()),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)
		response := testutils.GetJSONBody[cmkapi.LabelList](t, w)
		assert.Empty(t, response.Value)
	})
}

func TestLabelsController_GetKeyLabelsPagination(t *testing.T) {
	db, sv, tenant := startAPIKeyLabels(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	key := testutils.NewKey(func(_ *model.Key) {})
	for range totalRecordCount {
		key.KeyLabels = append(
			key.KeyLabels,
			*testutils.NewKeyLabel(func(_ *model.KeyLabel) {}),
		)
	}

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

	type testCase struct {
		name           string
		doesKeyExist   bool
		expectedStatus int
		query          string
		count          bool
		expectedCount  int
		expectedSize   int
	}

	tcs := []testCase{
		{
			name:           "GETLabelsPaginationDefaultValues",
			expectedStatus: http.StatusOK,
			doesKeyExist:   true,
			query:          "/key/%s/labels",
			count:          false,
			expectedSize:   20,
		},
		{
			name:           "GETLabelsPaginationWithCount",
			expectedStatus: http.StatusOK,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$count=true",
			count:          true,
			expectedCount:  totalRecordCount,
			expectedSize:   20,
		},
		{
			name:           "GETLabelsPaginationTopZero",
			expectedStatus: http.StatusBadRequest,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$top=0",
			count:          false,
		},
		{
			name:           "GETLabelsPaginationTopZeroWithCount",
			expectedStatus: http.StatusBadRequest,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$top=0&$count=true",
			count:          true,
		},
		{
			name:           "GETLabelsPaginationOnlyTopParam",
			expectedStatus: http.StatusOK,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$top=3",
			count:          false,
			expectedSize:   3,
		},
		{
			name:           "GETLabelsPaginationOnlyTopParamWithCount",
			expectedStatus: http.StatusOK,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$count=true&$top=25",
			count:          true,
			expectedCount:  totalRecordCount,
			expectedSize:   21,
		},
		{
			name:           "GETLabelsPaginationTopAndSkipParams",
			expectedStatus: http.StatusOK,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$top=17&$skip=23",
			count:          false,
			expectedCount:  totalRecordCount,
			expectedSize:   0,
		},
		{
			name:           "GETLabelsPaginationTopAndSkipParamsWithCount",
			expectedStatus: http.StatusOK,
			doesKeyExist:   true,
			query:          "/key/%s/labels?$top=17&$skip=23&$count=true",
			count:          true,
			expectedCount:  totalRecordCount,
			expectedSize:   0,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf(tc.query, key.ID.String()),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})

			assert.Equal(t, tc.expectedStatus, w.Code)

			if w.Code == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.LabelList](t, w)

				assert.Len(t, response.Value, tc.expectedSize)

				if tc.count {
					assert.Equal(t, tc.expectedCount, response.Count)
				} else {
					assert.Equal(t, 0, response.Count)
				}
			}
		})
	}
}

func TestLabelsController_CreateOrUpdateLabels(t *testing.T) {
	type testCase struct {
		name                         string
		inputLabels                  []cmkapi.Label
		doesKeyExist                 bool
		expectedStatus               int
		validateByFetchingDataFromDB bool
		updatedLabels                []cmkapi.Label
		expectedLabels               []cmkapi.Label
	}

	tcs := []testCase{
		{
			name:                         "Add_Duplicate_Labels",
			inputLabels:                  regularLabels,
			doesKeyExist:                 true,
			expectedStatus:               http.StatusNoContent,
			updatedLabels:                regularLabels,
			validateByFetchingDataFromDB: true,
			expectedLabels:               regularLabels,
		},
		{
			name:                         "Add_Labels_To_NonExisting_Key",
			inputLabels:                  regularLabels,
			doesKeyExist:                 false,
			expectedStatus:               http.StatusNotFound,
			validateByFetchingDataFromDB: false,
		},
		{
			name:           "Add_Labels_To_Existing_Key",
			inputLabels:    regularLabels,
			doesKeyExist:   true,
			expectedStatus: http.StatusNoContent,
			expectedLabels: regularLabels,
		},
		{
			name:           "Add_Empty_Labels_To_Key",
			inputLabels:    []cmkapi.Label{},
			doesKeyExist:   true,
			expectedStatus: http.StatusBadRequest,
			expectedLabels: []cmkapi.Label{},
		},
		{
			name:                         "Update_Label_Value_To_Empty_String",
			inputLabels:                  regularLabels,
			doesKeyExist:                 true,
			expectedStatus:               http.StatusNoContent,
			validateByFetchingDataFromDB: true,
			updatedLabels: []cmkapi.Label{
				{
					Key:   "foo",
					Value: ptr.PointTo(""),
				},
			},
			expectedLabels: []cmkapi.Label{
				{
					Key:   "foo",
					Value: ptr.PointTo(""),
				}, {
					Key:   "region/az",
					Value: ptr.PointTo("eu-west-1/a"),
				},
			},
		},
		{
			name:           "Add_Label_With_Empty_Value",
			inputLabels:    labelWithEmptyValue,
			doesKeyExist:   true,
			expectedStatus: http.StatusNoContent,
			expectedLabels: labelWithEmptyValue,
		},
		{
			name:           "Update_Existing_Labels",
			inputLabels:    regularLabels,
			doesKeyExist:   true,
			expectedStatus: http.StatusNoContent,
			updatedLabels: []cmkapi.Label{{
				Key:   "foo",
				Value: ptr.PointTo("updated-value"),
			}},
			expectedLabels: []cmkapi.Label{
				{
					Key:   "foo",
					Value: ptr.PointTo("updated-value"),
				}, {
					Key:   "region/az",
					Value: ptr.PointTo("eu-west-1/a"),
				},
			},
		},
		{
			name:           "Add_Labels_Payload_As_Invalid_Formatted_JSON",
			inputLabels:    errorLabel,
			doesKeyExist:   true,
			expectedStatus: http.StatusBadRequest,
		},
	}

	db, sv, tenant := startAPIKeyLabels(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var key *model.Key
			if tc.doesKeyExist {
				key = testutils.NewKey(func(k *model.Key) { k.KeyConfigurationID = keyConfig.ID })
				testutils.CreateTestEntities(ctx, t, r, key)
			} else {
				key = testutils.NewKey(func(k *model.Key) { k.KeyConfigurationID = keyConfig.ID })
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPost,
				Endpoint:          fmt.Sprintf(apiCreateOrUpdateLabelsFmt, key.ID.String()),
				Tenant:            tenant,
				Body:              testutils.WithJSON(t, tc.inputLabels),
				AdditionalContext: authClient.GetClientMap(),
			})

			if !slices.Equal(tc.updatedLabels, []cmkapi.Label{}) {
				w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          fmt.Sprintf(apiCreateOrUpdateLabelsFmt, key.ID.String()),
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, tc.updatedLabels),
					AdditionalContext: authClient.GetClientMap(),
				})
			}

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.validateByFetchingDataFromDB && tc.doesKeyExist {
				var ls []*model.KeyLabel

				err := r.List(ctx, model.KeyLabel{}, &ls, *repo.NewQuery().Order(repo.OrderField{
					Field:     "Key",
					Direction: repo.Asc,
				}).Where(
					repo.NewCompositeKeyGroup(
						repo.NewCompositeKey().Where(
							repo.ResourceIDField, key.ID),
					),
				))
				assert.NoError(t, err)

				for i, l := range tc.expectedLabels {
					assert.Equal(t, *l.Value, ls[i].Value)
					assert.Equal(t, l.Key, ls[i].Key)
				}
			}
		})
	}
}

func TestLabelsController_DeleteLabel(t *testing.T) {
	type testCase struct {
		name                         string
		doesKeyExist                 bool
		expectedStatus               int
		labelToBeDeleted             string
		validateByFetchingDataFromDB bool
		expectedLabels               []cmkapi.Label
	}

	tcs := []testCase{
		{
			name:                         "Delete_Label_From_NonExisting_Key",
			doesKeyExist:                 false,
			expectedStatus:               http.StatusNotFound,
			validateByFetchingDataFromDB: false,
		},
		{
			name:                         "Delete_NonExisting_Label",
			doesKeyExist:                 true,
			expectedStatus:               http.StatusNotFound,
			labelToBeDeleted:             "non-existing-label",
			validateByFetchingDataFromDB: true,
			expectedLabels:               regularLabels,
		},
		{
			name:                         "Delete_Existing_Label",
			doesKeyExist:                 true,
			expectedStatus:               http.StatusNoContent,
			labelToBeDeleted:             "foo",
			validateByFetchingDataFromDB: true,
			expectedLabels: []cmkapi.Label{{
				Key:   "region/az",
				Value: ptr.PointTo("eu-west-1/a"),
			}},
		},
	}

	db, sv, tenant := startAPIKeyLabels(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var key *model.Key

			if tc.doesKeyExist {
				keyID := uuid.New()
				key = testutils.NewKey(func(k *model.Key) {
					k.ID = keyID
					k.KeyLabels = []model.KeyLabel{
						*testutils.NewKeyLabel(func(l *model.KeyLabel) {
							l.Key = "foo"
							l.Value = "bar"
							l.ResourceID = keyID
						}),
						*testutils.NewKeyLabel(func(l *model.KeyLabel) {
							l.Key = "region/az"
							l.Value = "eu-west-1/a"
							l.ResourceID = keyID
						}),
					}
				})
				testutils.CreateTestEntities(ctx, t, r, key)
			} else {
				key = testutils.NewKey(func(_ *model.Key) {})
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodDelete,
				Endpoint:          fmt.Sprintf("/key/%s/label/%s", key.ID.String(), tc.labelToBeDeleted),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.validateByFetchingDataFromDB && tc.doesKeyExist {
				var ls []*model.KeyLabel

				err := r.List(ctx, model.KeyLabel{}, &ls, *repo.NewQuery().Order(repo.OrderField{
					Field:     "Key",
					Direction: repo.Asc,
				}).Where(
					repo.NewCompositeKeyGroup(
						repo.NewCompositeKey().Where(
							repo.ResourceIDField, key.ID),
					),
				))
				assert.NoError(t, err)

				for i, l := range tc.expectedLabels {
					assert.Equal(t, *l.Value, ls[i].Value)
					assert.Equal(t, l.Key, ls[i].Key)
				}
			}
		})
	}
}

func TestLabelsController_DeleteInvalidLabel(t *testing.T) {
	db, sv, tenant := startAPIKeyLabels(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	shortStr := strings.Repeat("A", 255)
	longStr := shortStr + "0"

	key := testutils.NewKey(func(_ *model.Key) {})

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

	type testCase struct {
		name                 string
		key                  string
		value                string
		expectedAddStatus    int
		expectedDeleteStatus int
	}

	tcs := []testCase{
		{
			name:                 "good label",
			key:                  shortStr,
			value:                shortStr,
			expectedAddStatus:    http.StatusNoContent,
			expectedDeleteStatus: http.StatusNoContent,
		},
		{
			name:                 "bad key",
			key:                  longStr,
			value:                shortStr,
			expectedAddStatus:    http.StatusBadRequest,
			expectedDeleteStatus: http.StatusBadRequest,
		},
		{
			name:                 "bad value",
			key:                  shortStr,
			value:                longStr,
			expectedAddStatus:    http.StatusBadRequest,
			expectedDeleteStatus: http.StatusNotFound,
		},
		{
			name:                 "bad label",
			key:                  longStr,
			value:                longStr,
			expectedAddStatus:    http.StatusBadRequest,
			expectedDeleteStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			label := []cmkapi.Label{
				{
					Key:   tc.key,
					Value: ptr.PointTo(tc.value),
				},
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPost,
				Endpoint:          fmt.Sprintf(apiCreateOrUpdateLabelsFmt, key.ID.String()),
				Tenant:            tenant,
				Body:              testutils.WithJSON(t, label),
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tc.expectedAddStatus, w.Code)

			if tc.expectedAddStatus == http.StatusBadRequest {
				assert.Contains(t, w.Body.String(), "maximum string length is 255")
			}

			w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodDelete,
				Endpoint:          fmt.Sprintf("/key/%s/label/%s", key.ID.String(), tc.key),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tc.expectedDeleteStatus, w.Code)

			if tc.expectedDeleteStatus == http.StatusBadRequest {
				assert.Contains(t, w.Body.String(), "maximum string length is 255")
			}
		})
	}
}
