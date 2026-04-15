//go:build !unit

package cmk_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
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

func startAPIKeyVersion(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config: config.Config{Database: dbCfg},
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
		k.KeyConfigurationID = keyConfig.ID
	})

	key2 := testutils.NewKey(func(k *model.Key) {
		k.State = string(cmkapi.KeyStateENABLED)
		k.KeyConfigurationID = keyConfig.ID
	})

	key1Version1 := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
		kv.KeyID = key1.ID
		kv.RotatedAt = time.Now().UTC().Add(-2 * time.Hour)
	})

	key2Version1 := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
		kv.KeyID = key2.ID
		kv.RotatedAt = time.Now().UTC().Add(-2 * time.Hour)
	})

	key2Version2 := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
		kv.KeyID = key2.ID
		kv.RotatedAt = time.Now().UTC()
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
		key                 *model.Key
		expectedKeyVersions []model.KeyVersion
		expectedStatus      int
	}{
		{
			name:                "GetKeyVersions_Success_ReturnKey1Version",
			keyID:               key1.ID.String(),
			key:                 key1,
			expectedStatus:      http.StatusOK,
			expectedKeyVersions: []model.KeyVersion{*key1Version1},
		},
		{
			name:                "GetKeyVersions_Success_ReturnKey2Version",
			keyID:               key2.ID.String(),
			key:                 key2,
			expectedStatus:      http.StatusOK,
			expectedKeyVersions: []model.KeyVersion{*key2Version2, *key2Version1},
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
		{
			name:           "GetKeyVersions_NotFound_NonExistentKey",
			keyID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
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
					expectedKV := tt.expectedKeyVersions[i]

					// Assert NativeID
					assert.Equal(t, expectedKV.NativeID, *keyVersion.NativeID)

					// Assert State matches parent key
					assert.NotNil(t, keyVersion.State)
					assert.Equal(t, cmkapi.KeyState(tt.key.State), *keyVersion.State)

					// Assert IsPrimary - first version should be primary (latest)
					assert.NotNil(t, keyVersion.IsPrimary)
					if i == 0 {
						assert.True(t, *keyVersion.IsPrimary, "First version should be primary")
					} else {
						assert.False(t, *keyVersion.IsPrimary, "Non-first versions should not be primary")
					}

					// Assert RotatedAt
					assert.NotNil(t, keyVersion.Metadata)
					assert.NotNil(t, keyVersion.Metadata.RotatedAt)
					assert.Equal(t, expectedKV.RotatedAt.Unix(), keyVersion.Metadata.RotatedAt.Unix())
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
			kv.KeyID = key.ID
			kv.CreatedAt = time.Now()
			kv.RotatedAt = time.Now().UTC().Add(time.Duration(i) * time.Second)
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

func TestKeyVersionController_GetKeyVersions_IsPrimaryWithPagination(t *testing.T) {
	db, sv, tenant := startAPIKeyVersion(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
		k.State = string(cmkapi.KeyStateENABLED)
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

	// Create 5 versions with increasing rotation times
	// Version indices: 0 (oldest) -> 4 (newest/primary)
	for i := range 5 {
		keyVersion := testutils.NewKeyVersion(func(kv *model.KeyVersion) {
			kv.KeyID = key.ID
			kv.RotatedAt = time.Now().UTC().Add(time.Duration(i) * time.Hour)
		})
		testutils.CreateTestEntities(ctx, t, r, keyVersion)
	}

	tests := []struct {
		name               string
		query              string
		expectedSize       int
		expectedPrimaryIdx int // Index in the returned page that should be primary
		description        string
	}{
		{
			name:               "FirstPage_PrimaryShouldBeFirst",
			query:              "/keys/%s/versions?$top=3&$skip=0",
			expectedSize:       3,
			expectedPrimaryIdx: 0,
			description:        "First page: latest version (version 4) should be marked as primary",
		},
		{
			name:               "SecondPage_NoPrimaryShouldBeMarked",
			query:              "/keys/%s/versions?$top=3&$skip=3",
			expectedSize:       2,
			expectedPrimaryIdx: -1, // No version should be primary on second page
			description:        "Second page: no version should be marked as primary",
		},
		{
			name:               "LargePageSize_OnlyFirstIsPrimary",
			query:              "/keys/%s/versions?$top=10",
			expectedSize:       5,
			expectedPrimaryIdx: 0,
			description:        "All versions in one page: only first (latest) should be primary",
		},
		{
			name:               "SkipFirstVersion_NoPrimary",
			query:              "/keys/%s/versions?$top=5&$skip=1",
			expectedSize:       4,
			expectedPrimaryIdx: -1,
			description:        "Skip the latest version: no version should be primary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf(tt.query, key.ID.String()),
				Tenant:            tenant,
				AdditionalContext: authClient.GetClientMap(),
			})

			assert.Equal(t, http.StatusOK, w.Code, tt.description)
			response := testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)

			keyVersions := response.Value
			assert.Len(t, keyVersions, tt.expectedSize)

			// Check isPrimary for each version
			primaryCount := 0
			for i, kv := range keyVersions {
				assert.NotNil(t, kv.IsPrimary, "IsPrimary should not be nil at index %d", i)
				if *kv.IsPrimary {
					primaryCount++
					assert.Equal(t, tt.expectedPrimaryIdx, i,
						"Primary version at index %d, expected at %d. %s",
						i, tt.expectedPrimaryIdx, tt.description)
				}
			}

			if tt.expectedPrimaryIdx == -1 {
				assert.Equal(t, 0, primaryCount,
					"No version should be marked as primary. %s", tt.description)
			} else {
				assert.Equal(t, 1, primaryCount,
					"Exactly one version should be marked as primary. %s", tt.description)
			}

			// Verify versions are sorted by RotatedAt descending
			if len(keyVersions) > 1 {
				for i := range len(keyVersions) - 1 {
					curr := keyVersions[i].Metadata.RotatedAt
					next := keyVersions[i+1].Metadata.RotatedAt
					assert.True(t, curr.After(*next) || curr.Equal(*next),
						"Versions should be sorted by RotatedAt descending")
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
				kv.KeyID = keyID
				kv.NativeID = uuid.NewString()
				kv.RotatedAt = time.Now().UTC()
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
		// NativeID should remain the same after disablement
		firstVersionNativeID := response.Value[0].NativeID

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
		// NativeID should remain the same after re-enablement
		assert.Equal(t, firstVersionNativeID, response.Value[0].NativeID)
	})
}

func TestKeyVersionController_GetKeyVersions_EmptyList(t *testing.T) {
	db, sv, tenant := startAPIKeyVersion(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {},
		testutils.WithAuthClientDataKC(authClient))

	// Create a key with NO versions
	keyWithNoVersions := testutils.NewKey(func(k *model.Key) {
		k.State = string(cmkapi.KeyStateENABLED)
		k.KeyConfigurationID = keyConfig.ID
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, keyWithNoVersions)

	t.Run("Should return empty list when key has no versions", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          fmt.Sprintf("/keys/%s/versions", keyWithNoVersions.ID),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)
		assert.NotNil(t, response.Value)
		assert.Empty(t, response.Value, "Should return empty list, not error")
	})

	t.Run("Should return count=0 when key has no versions and count is requested", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          fmt.Sprintf("/keys/%s/versions?$count=true", keyWithNoVersions.ID),
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyVersionList](t, w)
		assert.NotNil(t, response.Value)
		assert.Empty(t, response.Value)
		assert.NotNil(t, response.Count)
		assert.Equal(t, 0, *response.Count)
	})
}
