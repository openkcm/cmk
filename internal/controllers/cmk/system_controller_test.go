package cmk_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var ErrForced = errors.New("forced")

func startAPISystems(t *testing.T, cfg testutils.TestAPIServerConfig) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.System{},
			&model.SystemProperty{},
			&model.KeyConfiguration{},
			&model.Key{},
			&model.KeyVersion{},
			&model.KeyLabel{},
		},
	})

	sv := testutils.NewAPIServer(t, db, cfg)

	return db, sv, tenants[0]
}

func TestGetSystems_WithInvalidKeyConfigurationID(t *testing.T) {
	_, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})

	tests := []struct {
		name               string
		expectedStatus     int
		withKeyConfig      bool
		keyConfigurationID string
	}{
		{
			name:               "GetAllSystemsEmptyKeyConfigurationID",
			expectedStatus:     http.StatusBadRequest,
			keyConfigurationID: "",
		},
		{
			name:               "GetOneSystemsNonValidKeyConfigurationID",
			expectedStatus:     http.StatusBadRequest,
			keyConfigurationID: "test",
		},
		{
			name:               "GetOneSystemsNonExistingKeyConfigurationID",
			expectedStatus:     http.StatusNotFound,
			keyConfigurationID: uuid.New().String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/systems?$filter=keyConfigurationID eq '" + tt.keyConfigurationID + "'",
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestGetSystems_AdditionalProperties(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{
		Config: config.Config{
			ContextModels: config.ContextModels{
				System: config.System{
					OptionalProperties: map[string]config.SystemProperty{
						"test": {DisplayName: "test"},
					},
				},
			},
		},
	})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	systemWithProps := testutils.NewSystem(func(s *model.System) {
		s.Properties = map[string]string{
			"test": "test",
		}
	})
	systemWithoutProps := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		systemWithProps,
		systemWithoutProps,
	)

	t.Run("Should not show properties field on system without properties", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/systems/%s", systemWithoutProps.ID),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.System](t, w)
		assert.Nil(t, response.Properties)
	})

	t.Run("Should show properties field on system with properties", func(t *testing.T) {
		expected := &map[string]any{"test": "test"}
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/systems/%s", systemWithProps.ID),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.System](t, w)
		assert.Equal(t, expected, response.Properties)
	})
}

func TestGetSystems_WithKeyConfigurationID(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfiguration3ID := ptr.PointTo(uuid.New())

	keyConfig1 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	keyConfig2 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	systems1 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig1.ID)
	})
	systems2 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig2.ID)
	})
	systems3 := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig1,
		keyConfig2,
		systems1,
		systems2,
		systems3,
	)

	tests := []struct {
		name                 string
		expectedStatus       int
		withKeyConfig        bool
		keyConfigurationID   *uuid.UUID
		expectedSystemsCount int
		expectedSystems      []string
		expectedErrorCode    string
	}{
		{
			name:                 "Should get systems",
			expectedStatus:       http.StatusOK,
			keyConfigurationID:   nil,
			expectedSystemsCount: 3,
			expectedSystems:      []string{systems1.Identifier, systems2.Identifier, systems3.Identifier},
		},
		{
			name:                 "Should get systems filtered by keyConfigID",
			expectedStatus:       http.StatusOK,
			keyConfigurationID:   ptr.PointTo(keyConfig1.ID),
			expectedSystemsCount: 1,
			expectedSystems:      []string{systems1.Identifier},
		},
		{
			name:                 "Should error on getting systems filtered by non-existing keyConfigID",
			expectedStatus:       http.StatusNotFound,
			keyConfigurationID:   keyConfiguration3ID,
			expectedSystemsCount: 0,
			expectedSystems:      []string{},
			expectedErrorCode:    "KEY_CONFIGURATION_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/systems?$count=true"
			if tt.keyConfigurationID != nil {
				url = url + "&$filter=keyConfigurationID eq '" + tt.keyConfigurationID.String() + "'"
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: url,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus != w.Code {
				return
			}

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.SystemList](t, w)

				if len(tt.expectedSystems) != 0 {
					assert.NotEmpty(t, response.Value)
				}

				assert.Equal(t, tt.expectedSystemsCount, *response.Count)

				systems := response.Value
				assert.Len(t, systems, tt.expectedSystemsCount)

				identifiers := make([]string, 0, len(systems))
				for _, sys := range systems {
					identifiers = append(identifiers, *sys.Identifier)
				}

				assert.ElementsMatch(t, tt.expectedSystems, identifiers)
			} else {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedErrorCode, response.Error.Code)
			}
		})
	}
}

// TestAPIController_GetAllSystems tests the GetAllSystems function of SystemController
func TestAPIController_GetAllSystems(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})
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

	tests := []struct {
		name              string
		expectedStatus    int
		sideEffect        func() func()
		expectedErrorCode string
	}{
		{
			name:           "GetAllSystemsSuccess",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GetAllSystemsDbError",
			expectedStatus: http.StatusInternalServerError,
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithQuery().Register()

				return errForced.Unregister
			},
			expectedErrorCode: "QUERY_SYSTEM_LIST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/systems?$count=true",
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.SystemList](t, w)
				assert.Equal(t, 2, *response.Count)

				systems := response.Value
				assert.Len(t, systems, 2)

				ids := make([]uuid.UUID, 0, len(systems))
				for _, sys := range systems {
					ids = append(ids, *sys.ID)
				}

				assert.ElementsMatch(t, []uuid.UUID{system1.ID, system2.ID}, ids)
			} else {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedErrorCode, response.Error.Code)
			}
		})
	}
}

func TestAPIController_GetAllSystemsPagination(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	for range totalRecordCount {
		system := testutils.NewSystem(func(s *model.System) {
			s.Properties = map[string]string{
				"key-1": "val-1",
				"key-2": "val-2",
			}
		})
		testutils.CreateTestEntities(ctx, t, r, system)
	}

	tests := []struct {
		name               string
		query              string
		count              bool
		expectedStatus     int
		expectedCount      int
		expectedErrorCode  string
		expectedTotalCount int
	}{
		{
			name:           "GetAllSystemsDefaultPaginationValues",
			query:          "/systems",
			count:          false,
			expectedCount:  20,
			expectedStatus: http.StatusOK,
		},
		{
			name:               "GetAllSystemsDefaultPaginationValuesWithCount",
			query:              "/systems?$count=true",
			count:              true,
			expectedCount:      20,
			expectedStatus:     http.StatusOK,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:              "GetAllSystemsTopZero",
			query:             "/systems?$top=0",
			count:             false,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "VALIDATION_ERROR",
		},
		{
			name:              "GetAllSystemsTopZeroWithCount",
			query:             "/systems?$top=0&$count=true",
			count:             true,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "VALIDATION_ERROR",
		},
		{
			name:           "GETSystemsPaginationOnlyTopParam",
			query:          "/systems?$top=3",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:               "GETSystemsPaginationOnlyTopParamWithCount",
			query:              "/systems?$top=3&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      3,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETSystemsPaginationTopAndSkipParams",
			query:          "/systems?$skip=0&$top=10",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedCount:  10,
		},
		{
			name:               "GETSystemsPaginationTopAndSkipParamsWithCount",
			query:              "/systems?$skip=0&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETSystemsPaginationTopAndSkipParamsLast",
			query:          "/systems?$skip=20&$top=10",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:               "GETSystemsPaginationTopAndSkipParamsLastWithCount",
			query:              "/systems?$skip=20&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      1,
			expectedTotalCount: totalRecordCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.SystemList](t, w)

				if tt.count {
					assert.Equal(t, tt.expectedTotalCount, *response.Count)
				}

				assert.Len(t, response.Value, tt.expectedCount)
			} else {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedErrorCode, response.Error.Code)
			}
		})
	}
}

// TestAPIController_GetSystemByID tests the GetSystemByID function of SystemController
func TestAPIController_GetSystemByID(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	system := testutils.NewSystem(func(_ *model.System) {})
	systemInvalidKeyConfig := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(uuid.New())
	})
	testutils.CreateTestEntities(ctx, t, r, system, systemInvalidKeyConfig)

	tests := []struct {
		name              string
		id                string
		expectedStatus    int
		expectedErrorCode string
	}{
		{
			name:           "SystemGETByIdSuccess",
			expectedStatus: http.StatusOK,
			id:             system.ID.String(),
		},
		{
			name:              "SystemGETByIdInvalidId",
			expectedStatus:    http.StatusBadRequest,
			id:                "invalid-id",
			expectedErrorCode: apierrors.ParamsErr,
		},
		{
			name:              "SystemGETByIdNotFound",
			expectedStatus:    http.StatusNotFound,
			id:                uuid.NewString(),
			expectedErrorCode: "GET_SYSTEM_BY_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/systems/" + tt.id,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.System](t, w)

				assert.Equal(t, &system.ID, response.ID)
				assert.Equal(t, system.Identifier, *response.Identifier)
			} else {
				var response *cmkapi.ErrorMessage

				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedErrorCode, response.Error.Code)
			}
		})
	}
}

func TestAPIController_GetSystemByIDWithError(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	system := testutils.NewSystem(func(_ *model.System) {})
	testutils.CreateTestEntities(ctx, t, r, system)

	forced := testutils.NewDBErrorForced(db, ErrForced)

	forced.Register()
	defer forced.Unregister()

	tests := []struct {
		name              string
		id                string
		expectedStatus    int
		expectedErrorCode string
	}{
		{
			name:              "SystemGETByIdDbError",
			expectedStatus:    http.StatusInternalServerError,
			id:                system.ID.String(),
			expectedErrorCode: "GET_SYSTEM_BY_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/systems/" + tt.id,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
			assert.Equal(t, tt.expectedErrorCode, response.Error.Code)
		})
	}
}

// TestGetSystemLinkByID tests the GetSystemLinkByID function of SystemController
func TestAPIController_GetSystemLinkByID(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfiguration := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	system := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfiguration.ID)
	})
	systemWithoutKey := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfiguration,
		system,
		systemWithoutKey,
	)

	tests := []struct {
		name               string
		id                 string
		keyConfigurationID string
		expectedStatus     int
		expectedCode       string
	}{
		{
			name:               "SystemLinkGETByIdSuccess",
			expectedStatus:     http.StatusOK,
			id:                 system.ID.String(),
			keyConfigurationID: keyConfiguration.ID.String(),
		},
		{
			name:           "SystemLinkGETByIdNotFound",
			expectedStatus: http.StatusNotFound,
			expectedCode:   "GETTING_SYSTEM_LINK_BY_ID",
			id:             uuid.NewString(),
		},
		{
			name:           "SystemLinkGETByIdNoKeyConfig",
			expectedStatus: http.StatusNotFound,
			expectedCode:   "KEY_CONFIGURATION_ID_NOT_FOUND",
			id:             systemWithoutKey.ID.String(),
		},
		{
			name:           "SystemLinkGETByIdInvalidId",
			expectedCode:   apierrors.ParamsErr,
			expectedStatus: http.StatusBadRequest,
			id:             "invalid uuid",
		},
		{
			name:           "SystemLinkGETByIdDbError",
			expectedCode:   "GETTING_SYSTEM_LINK_BY_ID",
			expectedStatus: http.StatusInternalServerError,
			id:             system.ID.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedStatus == http.StatusInternalServerError {
				forced := testutils.NewDBErrorForced(db, ErrForced)

				forced.WithQuery().Register()
				defer forced.Register()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: fmt.Sprintf("/systems/%s/link", tt.id),
				Tenant:   tenant,
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.SystemLink](t, w)
				assert.Equal(t, tt.keyConfigurationID, response.KeyConfigurationID.String())
			} else {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedCode, response.Error.Code)
			}
		})
	}
}

// TestUpdateSystemByExternalID tests the UpdateSystemByExternalID function of SystemController
func TestAPIController_PatchSystemLinkByID(t *testing.T) {
	systemService := systems.NewFakeService(testutils.SetupLoggerWithBuffer())

	_, grpcCon := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
		},
	)

	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{
		Plugins: []testutils.MockPlugin{testutils.SystemInfo},
		GRPCCon: grpcCon,
	})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig1 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	keyConfig2 := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	})

	system := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig1.ID)
	})
	systemNoConfig := testutils.NewSystem(func(_ *model.System) {})
	systemWithKey := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig2.ID)
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig1,
		keyConfig2,
		system,
		systemNoConfig,
		systemWithKey,
	)

	tests := []struct {
		name               string
		ID                 string
		Identifier         string
		KeyConfigurationID string
		inputJSON          string
		expectedStatus     int
		errorForced        *testutils.ErrorForced
		expectedErrorCode  string
	}{
		{
			name:               "SystemUPDATESuccess",
			ID:                 systemNoConfig.ID.String(),
			Identifier:         systemNoConfig.Identifier,
			KeyConfigurationID: keyConfig2.ID.String(),
			inputJSON:          fmt.Sprintf(`{"keyConfigurationID": "%s"}`, keyConfig2.ID.String()),
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "SystemUPDATESuccessAlreadyHasKeyConfig",
			ID:                 systemWithKey.ID.String(),
			Identifier:         systemWithKey.Identifier,
			KeyConfigurationID: keyConfig2.ID.String(),
			inputJSON:          fmt.Sprintf(`{"keyConfigurationID": "%s"}`, keyConfig2.ID.String()),
			expectedStatus:     http.StatusOK,
		},
		{
			name:              "SystemUPDATEIdWithInvalidKeyConfigurationUUID",
			ID:                "invalid UUID",
			inputJSON:         fmt.Sprintf(`{"keyConfigurationID": "%s"}`, keyConfig2.ID.String()),
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: apierrors.ParamsErr,
		},
		{
			name:              "SystemUPDATEEmptyKeyConfigurationId",
			ID:                system.ID.String(),
			inputJSON:         `{"keyConfigurationID": ""}`,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "JSON_DECODE_ERROR",
		},
		{
			name:              "SystemUPDATEMissingKeyConfigurationID",
			ID:                system.ID.String(),
			Identifier:        systemWithKey.Identifier,
			inputJSON:         `{}`,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "VALIDATION_ERROR",
		},
		{
			name:              "SystemUPDATEEmptyKeyConfiguration",
			ID:                system.ID.String(),
			Identifier:        systemWithKey.Identifier,
			inputJSON:         ``,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "VALIDATION_ERROR",
		},
		{
			name:              "SystemUPDATEIdGetDbError",
			ID:                system.ID.String(),
			inputJSON:         fmt.Sprintf(`{"keyConfigurationID": "%s"}`, keyConfig2.ID.String()),
			expectedStatus:    http.StatusInternalServerError,
			errorForced:       testutils.NewDBErrorForced(db, ErrForced).WithQuery(),
			expectedErrorCode: "GET_KEY_CONFIG_BY_ID",
		},
		{
			name:              "SystemUPDATEConfigWithoutPrimaryKey",
			ID:                system.ID.String(),
			inputJSON:         fmt.Sprintf(`{"keyConfigurationID": "%s"}`, keyConfig1.ID.String()),
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "INVALID_TARGET_STATE",
		},
		{
			name:              "SystemUPDATEIdUpdateDbError",
			ID:                system.ID.String(),
			inputJSON:         fmt.Sprintf(`{"keyConfigurationID": "%s"}`, keyConfig2.ID.String()),
			expectedStatus:    http.StatusInternalServerError,
			errorForced:       testutils.NewDBErrorForced(db, ErrForced).WithUpdate(),
			expectedErrorCode: "UPDATE_SYSTEM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errorForced != nil {
				tt.errorForced.Register()
				defer tt.errorForced.Unregister()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPatch,
				Endpoint: fmt.Sprintf("/systems/%s/link", tt.ID),
				Tenant:   tenant,
				Body:     testutils.WithString(t, tt.inputJSON),
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.System](t, w)

				assert.Equal(t, tt.ID, response.ID.String())
				assert.Equal(t, tt.Identifier, *response.Identifier)
				assert.Equal(t, cmkapi.SystemStatusPROCESSING, response.Status)

				if tt.KeyConfigurationID != "" {
					configurationID := response.KeyConfigurationID.String()
					assert.Equal(t, tt.KeyConfigurationID, configurationID)
				} else {
					assert.Nil(t, response.KeyConfigurationID)
				}
			} else {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedErrorCode, response.Error.Code)
			}
		})
	}
}

// DeleteSystemLinkByID tests the DeleteSystemLinkByID function of SystemController
func TestAPIController_DeleteSystemLinkByID(t *testing.T) {
	db, sv, tenant := startAPISystems(t, testutils.TestAPIServerConfig{
		Plugins: []testutils.MockPlugin{testutils.SystemInfo},
	})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) { k.PrimaryKeyID = ptr.PointTo(uuid.New()) })
	system := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
	})
	systemWithoutKey := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, system, systemWithoutKey)

	tests := []struct {
		name              string
		id                uuid.UUID
		expectedStatus    int
		expectedErrorCode string
	}{
		{
			name:              "SystemLinkNoSystem",
			expectedStatus:    http.StatusNotFound,
			id:                uuid.New(),
			expectedErrorCode: "GET_SYSTEM_ID",
		},
		{
			name:           "SystemLinkDELETENoKeyConfig",
			expectedStatus: http.StatusBadRequest,
			id:             systemWithoutKey.ID,
		},
		{
			name:              "SystemLinkDELETEIdDbError",
			expectedStatus:    http.StatusInternalServerError,
			id:                system.ID,
			expectedErrorCode: "GET_SYSTEM_ID",
		},
		{
			name:           "SystemLinkDELETESuccess",
			expectedStatus: http.StatusNoContent,
			id:             system.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedStatus == http.StatusInternalServerError {
				forced := testutils.NewDBErrorForced(db, ErrForced)

				forced.WithUpdate().Register()
				defer forced.Unregister()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodDelete,
				Endpoint: fmt.Sprintf("/systems/%s/link", tt.id),
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if w.Code == http.StatusNoContent {
				system := &model.System{ID: tt.id}

				_, err := r.First(ctx, system, *repo.NewQuery())
				assert.NoError(t, err)

				assert.Nil(t, system.KeyConfigurationID)
			}
		})
	}
}
