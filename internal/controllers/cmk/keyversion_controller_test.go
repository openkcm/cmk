//go:build !unit

package cmk_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func startAPIKeyVersion(t *testing.T, plugins ...catalog.BuiltInPlugin) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Plugins: plugins,
		Config:  config.Config{Database: dbCfg},
	}), tenants[0]
}

func TestKeyVersionController_GetKeyVersions(t *testing.T) {
	db, sv, tenant := startAPIKeyVersion(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	key1 := testutils.NewKey(func(k *model.Key) {
		k.CreatedAt = time.Now()
		k.State = string(cmkapi.KeyStateENABLED)
	})

	key2 := testutils.NewKey(func(k *model.Key) {
		k.State = string(cmkapi.KeyStateENABLED)
	})

	key1Version1 := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
		kv.Version = 1
		kv.Key = *key1
		kv.KeyID = key1.ID
	})

	key2Version1 := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
		kv.Version = 1
		kv.Key = *key2
		kv.KeyID = key2.ID
	})

	key2Version2 := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
		kv.Version = 2
		kv.Key = *key2
		kv.KeyID = key2.ID
	})

	testutils.CreateTestEntities(ctx, t, r,
		keyConfig,
		key1,
		key2,
		key1Version1,
		key2Version1,
		key2Version2,
	)

	tests := []struct {
		name                string
		keyID               string
		expectedKeyVersions []model.KeyVersion
		expectedStatus      int
	}{
		{
			name:                "GetKeyVersions_Success_ReturnKey1Version",
			keyID:               key1.ID.String(),
			expectedStatus:      http.StatusOK,
			expectedKeyVersions: []model.KeyVersion{*key1Version1},
		},
		{
			name:                "GetKeyVersions_Success_ReturnKey2Version",
			keyID:               key2.ID.String(),
			expectedStatus:      http.StatusOK,
			expectedKeyVersions: []model.KeyVersion{*key2Version1, *key2Version2},
		},
		{
			name:           "GetKeyVersions_Success_ReturnEmpty",
			keyID:          " ",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GetKeyVersions_Error",
			keyID:          "30",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf("/keys/%s/versions", tt.keyID),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)

				keyVersions := response.Value
				assert.Len(t, keyVersions, len(tt.expectedKeyVersions))

				for i, keyVersion := range keyVersions {
					assert.Equal(t, tt.expectedKeyVersions[i].Version, *keyVersion.Version)
				}
			}
		})
	}
}

func TestKeyVersionController_GetKeyVersionsPagination(t *testing.T) {
	db, sv, tenant := startAPIKeyVersion(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))
	key := testutils.NewKey(func(k *model.Key) { k.KeyConfigurationID = keyConfig.ID })
	testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

	for i := range totalRecordCount {
		keyVersion := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
			kv.Version = i
			kv.Key = *key
			kv.KeyID = key.ID
			kv.CreatedAt = time.Now()
		})
		testutils.CreateTestEntities(ctx, t, r, keyVersion)
	}

	tests := []struct {
		name               string
		keyID              string
		expectedStatus     int
		query              string
		count              bool
		expectedSize       int
		expectedErrorCode  string
		expectedTotalCount int
	}{
		{
			name:           "GetKeyVersionsDefaultPaginationValues",
			keyID:          key.ID.String(),
			expectedStatus: http.StatusOK,
			query:          "/keys/%s/versions",
			count:          false,
			expectedSize:   20,
		},
		{
			name:               "GetKeyVersionsDefaultPaginationValuesWithCount",
			keyID:              key.ID.String(),
			expectedStatus:     http.StatusOK,
			query:              "/keys/%s/versions?$count=true",
			count:              true,
			expectedSize:       20,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:              "GetAllKeyVersionsTopZero",
			keyID:             key.ID.String(),
			query:             "/keys/%s/versions?$top=0&$count=true",
			count:             true,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "VALIDATION_ERROR",
		},
		{
			name:           "GETKeyVersionsPaginationOnlyTopParam",
			keyID:          key.ID.String(),
			query:          "/keys/%s/versions?$top=3",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedSize:   3,
		},
		{
			name:               "GETKeyVersionsPaginationOnlyTopParamWithCount",
			keyID:              key.ID.String(),
			query:              "/keys/%s/versions?$top=3&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedSize:       3,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GETKeyVersionsPaginationTopAndSkipParams",
			keyID:              key.ID.String(),
			query:              "/keys/%s/versions?$skip=0&$top=10",
			count:              false,
			expectedStatus:     http.StatusOK,
			expectedSize:       10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GETKeyVersionsPaginationTopAndSkipParamsWithCount",
			keyID:              key.ID.String(),
			query:              "/keys/%s/versions?$skip=0&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedSize:       10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETKeyVersionsPaginationTopAndSkipParamsLast",
			keyID:          key.ID.String(),
			query:          "/keys/%s/versions?$skip=20&$top=10",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedSize:   1,
		},
		{
			name:               "GETKeyVersionsPaginationTopAndSkipParamsLastWithCount",
			keyID:              key.ID.String(),
			query:              "/keys/%s/versions?$skip=20&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedSize:       1,
			expectedTotalCount: totalRecordCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf(tt.query, tt.keyID),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)

				keyVersions := response.Value
				assert.Len(t, keyVersions, tt.expectedSize)

				if tt.count {
					assert.Equal(t, tt.expectedTotalCount, *response.Count)
				} else {
					assert.Nil(t, response.Count)
				}
			}
		})
	}
}

func TestKeyVersionRefreshAndDisable(t *testing.T) {
	db, sv, tenant := startAPIKeys(t, testplugins.NewKeystoreOperator())
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	keyID := uuid.New()
	key := testutils.NewKey(func(k *model.Key) {
		k.ID = keyID
		k.Provider = providerTest
		k.State = string(cmkapi.KeyStateENABLED)
		k.KeyConfigurationID = keyConfig.ID
		k.KeyVersions = []model.KeyVersion{
			*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
				kv.Version = 1
				kv.IsPrimary = true
				kv.KeyID = keyID
				kv.Key.ID = keyID
				kv.NativeID = ptr.PointTo(uuid.NewString())
			}),
		}
		k.NativeID = ptr.PointTo(uuid.NewString())
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		key,
		keyConfig,
		keystore,
		keystoreDefaultCert,
	)

	t.Run("Re-enabling key should restore enabling and previous state", func(t *testing.T) {
		// Disable Key
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodPatch,
			Endpoint:          fmt.Sprintf("/keys/%s", key.ID),
			Tenant:            tenant,
			Body:              testutils.WithString(t, `{"enabled": false}`),
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)

		// Get key versions
		w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          fmt.Sprintf("/keys/%s/versions", key.ID),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)
		// Version enablement should be as before disablement
		assert.Equal(t, *response.Value[0].Version, key.MaxVersion())

		// Enable Key
		w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodPatch,
			Endpoint:          fmt.Sprintf("/keys/%s", key.ID),
			Tenant:            tenant,
			Body:              testutils.WithString(t, `{"enabled": true}`),
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)

		// Get key versions
		w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          fmt.Sprintf("/keys/%s/versions", key.ID),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		assert.Equal(t, http.StatusOK, w.Code)

		response = testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)
		// Version enablement should be as before disablement
		assert.Equal(t, *response.Value[0].Version, key.MaxVersion())
	})
}

func TestKeyVersionController_GetKeyVersionAndNumber(t *testing.T) {
	db, sv, tenant := startAPIKeyVersion(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	key1ID := uuid.New()
	key1 := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
		k.KeyVersions = []model.KeyVersion{
			*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
				kv.Version = 1
				kv.IsPrimary = true
				kv.Key.ID = key1ID
			}),
		}
		k.ID = key1ID
	})

	key2ID := uuid.New()
	key2 := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
		k.KeyVersions = []model.KeyVersion{
			*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
				kv.Version = 1
				kv.IsPrimary = false
				kv.Key.ID = key2ID
			}),
			*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
				kv.Version = 2
				kv.IsPrimary = true
				kv.Key.ID = key2ID
			}),
		}
		k.ID = key2ID
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		key1,
		key2,
	)

	tests := []struct {
		name               string
		key                model.Key
		inputValue         string
		expectedKeyVersion model.KeyVersion
		expectedStatus     int
	}{
		{
			name:               "GetKeyVersionByNumber_Success_ReturnKey1Version1",
			key:                *key1,
			inputValue:         "1",
			expectedStatus:     http.StatusOK,
			expectedKeyVersion: key1.KeyVersions[0],
		},
		{
			name:               "GetKeyVersionByNumber_Success_ReturnKey2Version1",
			key:                *key2,
			inputValue:         "1",
			expectedStatus:     http.StatusOK,
			expectedKeyVersion: key2.KeyVersions[0],
		},
		{
			name:               "GetKeyVersionByNumber_Success_ReturnKey2Version2",
			key:                *key2,
			inputValue:         "2",
			expectedStatus:     http.StatusOK,
			expectedKeyVersion: key2.KeyVersions[1],
		},
		{
			name:               "GetKeyVersionByNumber_Success_ReturnLatest",
			key:                *key2,
			inputValue:         "latest",
			expectedStatus:     http.StatusOK,
			expectedKeyVersion: key2.KeyVersions[1],
		},
		{
			name:           "GetKeyVersionByNumber_InternalServerError",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf("/keys/%s/versions/%s", tt.key.ID.String(), tt.inputValue),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.KeyVersion](t, w)
				assert.Equal(t, tt.expectedKeyVersion.Version, *response.Version)
			}
		})
	}
}
