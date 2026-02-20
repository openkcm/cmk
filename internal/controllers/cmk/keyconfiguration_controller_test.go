//go:build !unit

package cmk_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/crypto"
	"github.com/openkcm/cmk/utils/ptr"
)

func startAPIKeyConfigBase(t *testing.T,
	cfg testutils.TestAPIServerConfig) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	tenant := tenants[0]

	return db, testutils.NewAPIServer(t, db, cfg), tenant
}

func getContextAndRepo(t *testing.T, tenant string,
	db *multitenancy.DB) (context.Context, *sql.ResourceRepository) {
	t.Helper()
	return cmkcontext.CreateTenantContext(t.Context(), tenant), sql.NewRepository(db)
}

func startAPIKeyConfig(t *testing.T, cfg testutils.TestAPIServerConfig) (*multitenancy.DB,
	cmkapi.ServeMux, string, context.Context, *sql.ResourceRepository) {
	t.Helper()
	db, sv, tenant := startAPIKeyConfigBase(t, cfg)

	ctx, r := getContextAndRepo(t, tenant, db)

	return db, sv, tenant, ctx, r
}

func TestKeyConfigurationGetConfiguration(t *testing.T) {
	_, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	}, testutils.WithAuthClientDataKC(authClient))

	authClient2 := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig2 := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	}, testutils.WithAuthClientDataKC(authClient2))

	keyConfig3 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig, keyConfig2, keyConfig3)

	t.Run("Should get keyConfig", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/keyConfigurations/" + keyConfig.ID.String(),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyConfiguration](t, w)

		assert.Equal(t, keyConfig.PrimaryKeyID, response.PrimaryKeyID)
		assert.Equal(t, keyConfig.Name, response.Name)
		assert.True(t, *response.CanConnectSystems)
	})

	t.Run("Should get keyConfigs with permissions", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations?$skip=0&$top=10&$count=true",
			Tenant:   tenant,
			AdditionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyConfigurationList](t, w)

		assert.Equal(t, 2, *response.Count)
	})

	t.Run("Should not get keyConfig without permissions", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/keyConfigurations?$skip=0&$top=10&$count=true",
			Tenant:            tenant,
			AdditionalContext: testutils.GetGrouplessClientMap(),
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestKeyConfigurationGetConfigurationsWithGroups(t *testing.T) {
	_, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	}, testutils.WithAuthClientDataKC(authClient))
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should get keyConfig", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations?expandGroup=true",
			Tenant:   tenant,
			AdditionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		})
		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyConfigurationList](t, w)

		assert.Equal(t, keyConfig.PrimaryKeyID, response.Value[0].PrimaryKeyID)
		assert.Equal(t, keyConfig.Name, response.Value[0].Name)
		assert.Equal(t, keyConfig.AdminGroupID, response.Value[0].AdminGroupID)

		assert.Equal(t, authClient.Group.ID, *response.Value[0].AdminGroup.Id)
		assert.Equal(t, authClient.Group.Name, response.Value[0].AdminGroup.Name)
		assert.Equal(t, string(constants.KeyAdminRole),
			string(response.Value[0].AdminGroup.Role))

		assert.True(t, *response.Value[0].CanConnectSystems)
	})
}

func TestKeyconfigurationControllerGetKeyconfigurationsPagination(t *testing.T) {
	_, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	groups := make([]string, totalRecordCount)
	for i := range totalRecordCount {
		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {},
			testutils.WithAuthClientDataKC(authClient))
		testutils.CreateTestEntities(ctx, t, r, keyConfig)
		groups[i] = keyConfig.AdminGroup.IAMIdentifier
	}

	tests := []struct {
		name               string
		query              string
		count              bool
		expectedStatus     int
		expectedCount      int
		expectedTotalCount int
	}{
		{
			name:           "GETKeyConfigurationsPaginationDefaultValues",
			query:          "/keyConfigurations",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedCount:  20,
		},
		{
			name:               "GETKeyConfigurationsPaginationDefaultValuesWithCount",
			query:              "/keyConfigurations?$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      20,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETKeyConfigurationsPaginationTopZero",
			query:          "/keyConfigurations?$top=0",
			count:          false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GETKeyConfigurationsPaginationTopZeroWithCount",
			query:          "/keyConfigurations?$top=0&$count=true",
			count:          true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GETKeyConfigurationsPaginationOnlyTopParam",
			query:          "/keyConfigurations?$top=3",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:               "GETKeyConfigurationsPaginationOnlyTopParamWithCount",
			query:              "/keyConfigurations?$top=3&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      3,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETKeyConfigurationsPaginationTopAndSkipParams",
			query:          "/keyConfigurations?$skip=0&$top=10",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedCount:  10,
		},
		{
			name:               "GETKeyConfigurationsPaginationTopAndSkipParamsWithCount",
			query:              "/keyConfigurations?$skip=0&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      10,
			expectedTotalCount: totalRecordCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
				AdditionalContext: authClient.GetClientMap(
					testutils.WithAdditionalGroup(uuid.NewString())),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCount < 1 {
				return
			}

			response := testutils.GetJSONBody[cmkapi.KeyConfigurationList](t, w)

			if tt.count {
				assert.Equal(t, tt.expectedTotalCount, *response.Count)
			}

			assert.Len(t, response.Value, tt.expectedCount)
			assert.Nil(t, response.Value[0].AdminGroup)
		})
	}
}

func TestKeyConfigurationController_PostKeyConfigurations(t *testing.T) {
	db, sv, tenant := startAPIKeyConfigBase(t, testutils.TestAPIServerConfig{})
	ctx, r := getContextAndRepo(t, tenant, db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	expectedIdenfier := uuid.NewString()
	expectedEmail := "bob@"

	type testCase struct {
		name              string
		input             cmkapi.KeyConfiguration
		expectedStatus    int
		expectedCode      string
		expectedBody      string
		additionalContext map[any]any
	}

	tests := []testCase{
		{
			name: "KeyConfigPOST_Failed_WithoutClientDataIdentifier",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: authClient.Group.ID,
			},
			expectedStatus: http.StatusForbidden,
			expectedCode:   "FORBIDDEN",
			expectedBody:   "Forbidden",
		},
		{
			name: "KeyConfigPOST_Success_WithClientDataUserGroups",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config-2",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: authClient.Group.ID,
			},
			additionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups:     []string{"some-group", authClient.Group.IAMIdentifier},
					Identifier: expectedIdenfier,
					Email:      expectedEmail,
				},
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   "test-config-2",
		},
		{
			name: "KeyConfigPOST_Unauthorised_WithWrongClientDataUserGroups",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config-2",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: authClient.Group.ID,
			},
			additionalContext: testutils.GetInvalidClientMap(),
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "FORBIDDEN",
			expectedBody:      "error",
		},
		{
			name: "KeyConfigPOST_Unauthorised_WithEmptyClientDataUserGroups",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config-2",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: authClient.Group.ID,
			},
			additionalContext: testutils.GetGrouplessClientMap(),
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "FORBIDDEN",
			expectedBody:      "error",
		},
		{
			name: "KeyConfigPOST_MissingName",
			input: cmkapi.KeyConfiguration{
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: authClient.Group.ID,
			},
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name: "KeyConfigPOST_EmptyName",
			input: cmkapi.KeyConfiguration{
				Name:         "",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: authClient.Group.ID,
			},
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name: "KeyConfigPOST_MissingAdminGroupID",
			input: cmkapi.KeyConfiguration{
				Name:        "",
				Description: ptr.PointTo("test-config"),
			},
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name: "KeyConfigPOST_NonExistentAdminGroupID",
			input: cmkapi.KeyConfiguration{
				Name:         "",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: uuid.New(),
			},
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name: "KeyConfigPOST_DuplicateName",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config-2",
				Description:  ptr.PointTo("test-config-2"),
				AdminGroupID: authClient.Group.ID,
			},
			additionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
			expectedStatus: http.StatusConflict,
			expectedCode:   "UNIQUE_ERROR",
			expectedBody:   "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPost,
				Endpoint:          "/keyConfigurations",
				Tenant:            tenant,
				Body:              testutils.WithJSON(t, tt.input),
				AdditionalContext: tt.additionalContext,
			})
			assert.Equal(t, tt.expectedStatus, w.Code)
			body := w.Body.String()
			assert.Contains(t, body, tt.expectedBody)

			if w.Code == http.StatusCreated {
				assert.Contains(t, body, expectedIdenfier)
				assert.Contains(t, body, expectedEmail)
			}

			if tt.expectedCode != "" {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedCode, response.Error.Code)
			}
		})
	}
}

func TestKeyConfigurationController_UpdateByID(t *testing.T) {
	_, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	newAdminGroupID := uuid.New()

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))
	existingKeyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.Name = "existing-config"
	}, testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig, existingKeyConfig)

	type testCase struct {
		name              string
		configID          string
		inputJSON         string
		expectedStatus    int
		expectedBody      string
		expectedCode      string
		additionalContext map[any]any
		validate          func(*testing.T, *httptest.ResponseRecorder)
	}

	tests := []testCase{
		{
			name:     "KeyConfigPATCH_Success_WithoutClientDataUserGroups (backward compatibility)",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-config",
                "description": "updated description"
            }`,
			expectedStatus:    http.StatusOK,
			expectedBody:      "updated-config",
			additionalContext: authClient.GetClientMap(),
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()

				response := testutils.GetJSONBody[cmkapi.KeyConfiguration](t, w)
				assert.Equal(t, "updated-config", response.Name)
				assert.Equal(t, "updated description", *response.Description)
			},
		},
		{
			name:     "KeyConfigPATCH_NameOnly",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-name-only"
            }`,
			expectedStatus:    http.StatusOK,
			expectedBody:      "updated-name-only",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:     "KeyConfigPATCH_WithClientDataUserGroups",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-name-only-client-data"
            }`,
			expectedStatus: http.StatusOK,
			expectedBody:   "updated-name-only-client-data",
			additionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		},
		{
			name:     "KeyConfigPATCH_Unauthorised_WithWrongClientDataUserGroups",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-name-only-client-data"
            }`,
			expectedStatus:    http.StatusForbidden,
			expectedBody:      "error",
			expectedCode:      "FORBIDDEN",
			additionalContext: authClient.GetClientMap(testutils.WithOverriddenGroup(2)),
		},
		{
			name:     "KeyConfigPATCH_Unauthorised_WithEmptyClientDataUserGroups",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-name-only-client-data"
            }`,
			expectedStatus:    http.StatusForbidden,
			expectedBody:      "error",
			expectedCode:      "FORBIDDEN",
			additionalContext: testutils.GetGrouplessClientMap(),
		},
		{
			name:     "KeyConfigPATCH_DescriptionOnly",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "description": "updated description only"
            }`,
			expectedStatus:    http.StatusOK,
			expectedBody:      "updated description only",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:     "KeyConfigPATCH_EmptyName",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": ""
            }`,
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:     "KeyConfigPATCH_AdminGroupIDNotAllowed",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-config",
                "adminGroupID": "` + newAdminGroupID.String() + `"
            }`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
			validate: func(t *testing.T, _ *httptest.ResponseRecorder) {
				t.Helper()

				c := &model.KeyConfiguration{ID: keyConfig.ID}
				_, err := r.First(ctx, c, *repo.NewQuery())
				assert.NoError(t, err)
			},
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:     "KeyConfigPATCH_NameConflict",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "existing-config"
            }`,
			expectedStatus:    http.StatusConflict,
			expectedCode:      "UNIQUE_ERROR",
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:     "KeyConfigPATCH_InvalidID",
			configID: "invalid-uuid",
			inputJSON: `{
                "name": "updated-config"
				"adminGroupID": "invalid-id"
            }`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
		},
		{
			name:     "KeyConfigPATCH_NotFound",
			configID: uuid.New().String(),
			inputJSON: `{
                "name": "updated-config"
            }`,
			expectedStatus:    http.StatusNotFound,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:     "KeyConfigPATCH_InvalidJSON",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-config",
                invalid json
            }`,
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "error",
			additionalContext: authClient.GetClientMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPatch,
				Endpoint:          "/keyConfigurations/" + tt.configID,
				Tenant:            tenant,
				Body:              testutils.WithString(t, tt.inputJSON),
				AdditionalContext: tt.additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)

			if tt.expectedCode != "" {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedCode, response.Error.Code)
			}

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

func TestKeyConfigurationController_DeleteByID(t *testing.T) {
	_, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	keyConfig2 := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	keyConfigWithSystems := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfigWithSystems.ID)
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, keyConfigWithSystems, sys, keyConfig2)

	type testCase struct {
		name              string
		configID          string
		expectedStatus    int
		expectedCode      string
		additionalContext map[any]any
	}

	tests := []testCase{
		{
			name:           "DeleteKeyConfig_Deny_WithoutClientDataUserGroups",
			configID:       keyConfig.ID.String(),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:              "DeleteKeyConfig_Unauthorised_WithEmptyClientDataUserGroups",
			configID:          keyConfig2.ID.String(),
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "FORBIDDEN",
			additionalContext: testutils.GetGrouplessClientMap(),
		},
		{
			name:              "DeleteKeyConfig_Unauthorised_WithWrongClientDataUserGroups",
			configID:          keyConfig2.ID.String(),
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "FORBIDDEN",
			additionalContext: authClient.GetClientMap(testutils.WithOverriddenGroup(2)),
		},
		{
			name:           "DeleteKeyConfig_Authorised_WithClientDataUserGroups",
			configID:       keyConfig2.ID.String(),
			expectedStatus: http.StatusNoContent,
			additionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		},
		{
			name:           "DeleteKeyConfig_InvalidID",
			configID:       "invalid-id",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:              "Should 400 on deletion with connected systems",
			configID:          keyConfigWithSystems.ID.String(),
			expectedStatus:    http.StatusBadRequest,
			additionalContext: authClient.GetClientMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodDelete,
				Endpoint:          "/keyConfigurations/" + tt.configID,
				Tenant:            tenant,
				AdditionalContext: tt.additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedCode, response.Error.Code)
			}
		})
	}
}

func TestKeyConfigurationController_GetByID(t *testing.T) {
	_, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	type testCase struct {
		name              string
		configID          string
		expectedStatus    int
		expectedCode      string
		additionalContext map[any]any
	}

	tests := []testCase{
		{
			name:              "GetKeyConfig_Success",
			configID:          keyConfig.ID.String(),
			expectedStatus:    http.StatusOK,
			additionalContext: authClient.GetClientMap(),
		},
		{
			name:           "GetKeyConfig_InvalidID",
			configID:       "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GetKeyConfig_Authorised_WithClientDataUserGroups",
			configID:       keyConfig.ID.String(),
			expectedStatus: http.StatusOK,
			additionalContext: authClient.GetClientMap(
				testutils.WithAdditionalGroup(uuid.NewString())),
		},
		{
			name:              "GetKeyConfig_Unauthorised_WithEmptyClientDataUserGroups",
			configID:          keyConfig.ID.String(),
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "FORBIDDEN",
			additionalContext: testutils.GetGrouplessClientMap(),
		},
		{
			name:              "GetKeyConfig_Unauthorised_WithWrongClientDataUserGroups",
			configID:          keyConfig.ID.String(),
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "FORBIDDEN",
			additionalContext: authClient.GetClientMap(testutils.WithOverriddenGroup(2)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/keyConfigurations/" + tt.configID,
				Tenant:            tenant,
				AdditionalContext: tt.additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedCode, response.Error.Code)
			} else if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.KeyConfiguration](t, w)
				assert.Equal(t, keyConfig.ID, *response.Id)
				assert.Equal(t, keyConfig.PrimaryKeyID, response.PrimaryKeyID)
				assert.Equal(t, keyConfig.Name, response.Name)
				assert.Equal(t, authClient.Group.ID, response.AdminGroupID)
				assert.Equal(t, authClient.Group.ID, *response.AdminGroup.Id)
				assert.Equal(t, authClient.Group.Name, response.AdminGroup.Name)
				assert.Equal(t, string(constants.KeyAdminRole), string(response.AdminGroup.Role))
			}
		})
	}
}

func TestAPIController_GetCertificates(t *testing.T) {
	tests := []struct {
		name                string
		expectedStatus      int
		expectedError       string
		setupFunc           func(t *testing.T, db *multitenancy.DB, tenant string)
		expectedRecordCount int
		expectedRootCA      string
		expectedSubject     string
		expectedType        string
		disableAuthzMW      bool
	}{
		{
			name:                "Success - Multiple OUs Certificate",
			expectedStatus:      http.StatusOK,
			expectedRecordCount: 1,
			expectedRootCA:      testutils.TestCertURL,
			expectedSubject:     "CN=myCert,OU=EXAMPLE OU1/EXAMPLE OU2/EXAMPLE-OU3,O=EXAMPLE_O,L=LOCAL/CMK,C=DE",
			expectedType:        "TENANT_DEFAULT",
			setupFunc: func(t *testing.T, db *multitenancy.DB, tenant string) {
				t.Helper()

				r := sql.NewRepository(db)
				privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
				assert.NoError(t, err)

				ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

				certPEM := testutils.CreateCertificatePEM(t, &x509.CertificateRequest{
					Subject: pkix.Name{
						Country:            []string{"DE"},
						Organization:       []string{"EXAMPLE_O"},
						OrganizationalUnit: []string{"EXAMPLE OU1", "EXAMPLE OU2", "EXAMPLE-OU3"},
						Locality:           []string{"LOCAL/CMK"},
						CommonName:         "myCert",
					},
				}, privateKey)

				cert := testutils.NewCertificate(func(c *model.Certificate) {
					c.CommonName = "myCert"
					c.CertPEM = string(certPEM)
					c.Purpose = model.CertificatePurposeTenantDefault
				})

				err = r.Create(ctx, cert)
				require.NoError(t, err)
			},
		},
		{
			name:                "Success - Single OU Certificate",
			expectedStatus:      http.StatusOK,
			expectedRecordCount: 1,
			expectedRootCA:      testutils.TestCertURL,
			expectedSubject:     "CN=myCert,OU=EXAMPLE OU1,O=EXAMPLE_O,L=LOCAL/CMK,C=DE",
			expectedType:        "TENANT_DEFAULT",
			setupFunc: func(t *testing.T, db *multitenancy.DB, tenant string) {
				t.Helper()

				r := sql.NewRepository(db)
				privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
				assert.NoError(t, err)

				ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

				certPEM := testutils.CreateCertificatePEM(t, &x509.CertificateRequest{
					Subject: pkix.Name{
						Country:            []string{"DE"},
						Organization:       []string{"EXAMPLE_O"},
						OrganizationalUnit: []string{"EXAMPLE OU1"},
						Locality:           []string{"LOCAL/CMK"},
						CommonName:         "myCert",
					},
				}, privateKey)

				cert := testutils.NewCertificate(func(c *model.Certificate) {
					c.CommonName = "singleOuCert"
					c.CertPEM = string(certPEM)
					c.Purpose = model.CertificatePurposeTenantDefault
				})

				err = r.Create(ctx, cert)
				require.NoError(t, err)
			},
		},
		{
			name:                "Success - No OU Certificate",
			expectedStatus:      http.StatusOK,
			expectedRecordCount: 1,
			expectedRootCA:      testutils.TestCertURL,
			expectedSubject:     "CN=myCert,O=EXAMPLE_O,L=LOCAL/CMK,C=DE",
			expectedType:        "TENANT_DEFAULT",
			setupFunc: func(t *testing.T, db *multitenancy.DB, tenant string) {
				t.Helper()

				r := sql.NewRepository(db)
				privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
				assert.NoError(t, err)

				ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

				certPEM := testutils.CreateCertificatePEM(t, &x509.CertificateRequest{
					Subject: pkix.Name{
						Country:      []string{"DE"},
						Organization: []string{"EXAMPLE_O"},
						Locality:     []string{"LOCAL/CMK"},
						CommonName:   "myCert",
					},
				}, privateKey)

				cert := testutils.NewCertificate(func(c *model.Certificate) {
					c.CommonName = "noOuCert"
					c.CertPEM = string(certPEM)
					c.Purpose = model.CertificatePurposeTenantDefault
				})

				err = r.Create(ctx, cert)
				require.NoError(t, err)
			},
		},
		{
			name: "Failed - Database error",
			setupFunc: func(_ *testing.T, db *multitenancy.DB, _ string) {
				forced := testutils.NewDBErrorForced(db, ErrForced)
				forced.Register()
				t.Cleanup(func() {
					forced.Unregister()
				})
			},
			expectedStatus:      http.StatusForbidden,
			expectedError:       "FORBIDDEN",
			expectedRecordCount: 0,
			disableAuthzMW:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cryptoCerts := map[string]testutils.CryptoCert{
				"crypto-1": {
					Subject: tt.expectedSubject,
					RootCA:  tt.expectedRootCA,
				},
			}
			bytes, err := json.Marshal(cryptoCerts)
			assert.NoError(t, err)

			db, sv, tenant, ctx, r := startAPIKeyConfig(t, testutils.TestAPIServerConfig{
				Config: config.Config{
					CryptoLayer: config.CryptoLayer{
						CertX509Trusts: commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  string(bytes),
						},
					},
					Certificates: config.Certificates{
						RootCertURL: testutils.TestCertURL,
					},
				},
			})

			key1 := testutils.NewKey(func(_ *model.Key) {})

			authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

			keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
				c.PrimaryKeyID = &key1.ID
			}, testutils.WithAuthClientDataKC(authClient))

			testutils.CreateTestEntities(ctx, t, r, key1, keyConfig)

			if tt.setupFunc != nil {
				tt.setupFunc(t, db, tenant)
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf("/keyConfigurations/%s/certificates", keyConfig.ID.String()),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.ClientCertificates](t, w)
				assert.Equal(t, tt.expectedRecordCount, *response.TenantDefault.Count)

				if *response.TenantDefault.Count > 0 {
					assert.Equal(t, tt.expectedRootCA, response.TenantDefault.Value[0].RootCA)
					assert.Equal(t, tt.expectedSubject, response.TenantDefault.Value[0].Subject)
				}
			} else {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedError, response.Error.Code)
			}
		})
	}
}
