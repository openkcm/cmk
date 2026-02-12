//go:build !unit

package cmk_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	keystore            = testutils.NewKeystore(func(_ *model.Keystore) {})
	keystoreDefaultCert = testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeKeystoreDefault
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
)

func startAPIKeys(t *testing.T, plugins ...testutils.MockPlugin) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})

	return db, testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Plugins: plugins,
		Config:  config.Config{Database: dbCfg},
	}), tenants[0]
}

func TestKeyControllerGetKeys(t *testing.T) {
	db, sv, tenant := startAPIKeys(t)
	nativeID := "arn:aws:kms:us-west-2:111122223333:alias/<alias-name>"

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key1 := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	key2 := testutils.NewKey(func(_ *model.Key) {})
	key3 := testutils.NewKey(func(k *model.Key) { k.NativeID = &nativeID })

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		key1,
		key2,
		key3,
	)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedLength int
		expectedNames  []string
	}{
		{
			name:           "Should fail on get keys without keyConfigurationID filter",
			query:          "/keys?$count=true",
			expectedStatus: http.StatusBadRequest,
			expectedLength: 0,
		},
		{
			name:           "Should get keys filtered by id",
			query:          fmt.Sprintf("/keys?keyConfigurationID=%s&$count=true", key1.KeyConfigurationID),
			expectedStatus: http.StatusOK,
			expectedLength: 1,
			expectedNames:  []string{key1.Name},
		},
		{
			name:           "Should fail on get keys filtered by non existing id",
			query:          "/keys?keyConfigurationID=" + uuid.New().String() + "&$count=true",
			expectedStatus: http.StatusNotFound,
			expectedLength: 0,
		},
		{
			name:           "Should fail on get keys filtered by invalid id",
			query:          "/keys?keyConfigurationID=a&$count=true",
			expectedStatus: http.StatusBadRequest,
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Groups: []string{keyConfig.AdminGroup.IAMIdentifier},
					},
				},
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedLength < 1 {
				return
			}

			response := testutils.GetJSONBody[cmkapi.KeyList](t, w)
			assert.Equal(t, &tt.expectedLength, response.Count)
			assert.Len(t, response.Value, tt.expectedLength)

			for _, key := range response.Value {
				assert.Contains(t, tt.expectedNames, key.Name)
			}
		})
	}
}

func TestKeyControllerGetKeysPagination(t *testing.T) {
	db, sv, tenant := startAPIKeys(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	for range totalRecordCount {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctx, t, r, key)
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
			name:               "GETKeysPaginationDefaultValues",
			query:              "/keys?keyConfigurationID=" + keyConfig.ID.String(),
			count:              false,
			expectedStatus:     http.StatusOK,
			expectedCount:      20,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GETKeysPaginationDefaultValuesWithCount",
			query:              "/keys?$count=true&keyConfigurationID=" + keyConfig.ID.String(),
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      20,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETKeysPaginationTopZero",
			query:          "/keys?$top=0&keyConfigurationID=" + keyConfig.ID.String(),
			count:          false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GETKeysPaginationTopZeroWithCount",
			query:          "/keys?$top=0&$count=true&keyConfigurationID=" + keyConfig.ID.String(),
			count:          true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:               "GETKeysPaginationOnlyTopParam",
			query:              "/keys?$top=3&keyConfigurationID=" + keyConfig.ID.String(),
			count:              false,
			expectedStatus:     http.StatusOK,
			expectedCount:      3,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GETKeysPaginationOnlyTopParamWithCount",
			query:              "/keys?$top=3&$count=true&keyConfigurationID=" + keyConfig.ID.String(),
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      3,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GETKeysPaginationTopAndSkipParams",
			query:              "/keys?$skip=0&$top=10&keyConfigurationID=" + keyConfig.ID.String(),
			count:              false,
			expectedStatus:     http.StatusOK,
			expectedCount:      10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GETKeysPaginationTopAndSkipParamsWithCount",
			query:              "/keys?$skip=0&$top=10&$count=true&keyConfigurationID=" + keyConfig.ID.String(),
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedCount:      10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GETKeysPagination Should fail on get keys filtered by non existing id",
			query:          "/keys?keyConfigurationID=" + uuid.New().String() + "&$skip=0&$top=10",
			count:          false,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "GETKeysPagination Should fail on get keys filtered by invalid id",
			query:          "/keys?keyConfigurationID=a",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Groups: []string{keyConfig.AdminGroup.IAMIdentifier},
					},
				},
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCount < 1 {
				return
			}

			response := testutils.GetJSONBody[cmkapi.KeyList](t, w)

			if tt.count {
				assert.Equal(t, tt.expectedTotalCount, *response.Count)
			}
		})
	}
}

func TestKeyControllerPostKeys(t *testing.T) {
	db, sv, tenant := startAPIKeys(t, testutils.KeyStorePlugin)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	r := sql.NewRepository(db)

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		tenantDefaultCert,
		keyConfig,
		keystore,
		keystoreDefaultCert,
	)

	SystemManagedRequest := map[string]any{
		"name":               "test-key",
		"type":               string(cmkapi.KeyTypeSYSTEMMANAGED),
		"keyConfigurationID": keyConfig.ID,
		"provider":           providerTest,
		"algorithm":          string(cmkapi.KeyAlgorithmAES256),
		"region":             "us-west-2",
		"description":        "test key",
		"enabled":            true,
	}

	HYOKRequest := map[string]any{
		"name":               "hyok-key",
		"type":               string(cmkapi.KeyTypeHYOK),
		"keyConfigurationID": keyConfig.ID,
		"enabled":            true,
		"nativeID":           "arn:aws:kms:eu-west-2:399521560603:key/03e6b16b-f0c8-4699-8ef9-8947871924d3",
		"provider":           providerTest,
		"hyokInfo": map[string]any{
			"trustAnchorArn": "arn:aws:rolesanywhere:eu-west-2:399521560603:trust-anchor/fe12790d-3695-4fbd-9150-64342ead324c",
			"readAccessRole": map[string]string{
				"roleArn":    "arn:aws:iam::399521560603:role/KMSServiceRoleAnywhere",
				"profileArn": "arn:aws:rolesanywhere:eu-west-2:399521560603:profile/b205661b-1e50-4910-be55-0a616293bd06",
			},
			"cryptoAccessRoles": []map[string]string{
				{
					"roleArn":    "arn:aws:iam::399521560603:role/KMSServiceRoleAnywhere",
					"profileArn": "arn:aws:rolesanywhere:eu-west-2:399521560603:profile/b205661b-1e50-4910-be55-0a616293bd06",
				},
			},
		},
	}

	// Create the mutator function
	requestMut := testutils.NewMutator(func() map[string]any {
		// Create a copy of the base map
		baseMap := make(map[string]any)
		maps.Copy(baseMap, SystemManagedRequest)

		return baseMap
	})

	// Create the mutator function
	hyokMut := testutils.NewMutator(func() map[string]any {
		// Create a deep copy of the base map
		baseMap := make(map[string]any)

		for k, v := range HYOKRequest {
			if nestedMap, ok := v.(map[string]any); ok {
				// Deep copy for nested maps
				copiedNestedMap := make(map[string]any)
				maps.Copy(copiedNestedMap, nestedMap)

				baseMap[k] = copiedNestedMap
			} else {
				baseMap[k] = v
			}
		}

		return baseMap
	})

	tests := []struct {
		name            string
		inputMap        map[string]any
		expectedStatus  int
		expectedMessage string
	}{
		{
			name:           "POST Key Success - enabled true",
			inputMap:       requestMut(),
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "POST Key Fail - duplicate name",
			inputMap:       requestMut(),
			expectedStatus: http.StatusConflict,
		},
		{
			name: "POST Key Success - enabled false",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-enabled-false"
				(*m)["enabled"] = false
			}),
			expectedStatus: http.StatusCreated,
		},
		{
			name: "POST Key Fail - missing name",
			inputMap: requestMut(func(m *map[string]any) {
				delete(*m, "name")
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - empty name",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = ""
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - missing type",
			inputMap: requestMut(func(m *map[string]any) {
				delete(*m, "type")
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - type value is not one of the allowed",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["type"] = ""
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - missing keyConfigurationID",
			inputMap: requestMut(func(m *map[string]any) {
				delete(*m, "keyConfigurationID")
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - empty keyConfigurationID",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["keyConfigurationID"] = ""
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - invalid keyConfigurationID",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["keyConfigurationID"] = "6a90b766-86bf-4b9e-a19e-fea8e9ca080xdf"
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - invalid algorithm",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["algorithm"] = "invalid-algorithm"
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - missing algorithm",
			inputMap: requestMut(func(m *map[string]any) {
				delete(*m, "algorithm")
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - missing region",
			inputMap: requestMut(func(m *map[string]any) {
				delete(*m, "region")
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST Key Fail - native key",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["nativeID"] = uuid.New().String()
			}),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "POST HYOK Key Failed - Missing provider",
			inputMap: hyokMut(func(m *map[string]any) {
				delete(*m, "provider")
			}),
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "Field is missing: provider",
		},
		{
			name: "POST HYOK Key Failed - Unexpected algorithm",
			inputMap: hyokMut(func(m *map[string]any) {
				(*m)["algorithm"] = "AES256"
			}),
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "Property is unexpected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPost,
				Endpoint: "keys",
				Tenant:   tenant,
				Body:     testutils.WithJSON(t, tt.inputMap),
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Groups: []string{keyConfig.AdminGroup.IAMIdentifier},
					},
				},
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if w.Code != http.StatusCreated {
				testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
			}

			if tt.expectedMessage != "" {
				assert.Contains(t, w.Body.String(), tt.expectedMessage)
			}
		})
	}
}

func TestKeyControllerPostKeysDrainedKeystorePool(t *testing.T) {
	db, sv, tenant := startAPIKeys(t, testutils.KeyStorePlugin)
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should fail to create system managed key if keystore pool is drained", func(t *testing.T) {
		// Arrange
		sysManagedKey := map[string]any{
			"name":               "test-key",
			"type":               string(cmkapi.KeyTypeSYSTEMMANAGED),
			"keyConfigurationID": keyConfig.ID,
			"algorithm":          string(cmkapi.KeyAlgorithmAES256),
			"region":             "us-west-2",
			"description":        "test key",
			"enabled":            true,
		}
		// Act
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: "keys",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, sysManagedKey),
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups: []string{keyConfig.AdminGroup.IAMIdentifier},
				},
			},
		})
		// Assert
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "KEYSTORE_POOL_DRAINED", response.Error.Code)
	})

	t.Run("Should fail to create BYOK key if keystore pool is drained", func(t *testing.T) {
		// Arrange
		byokKey := map[string]any{
			"name":               "test-key",
			"type":               string(cmkapi.KeyTypeBYOK),
			"keyConfigurationID": keyConfig.ID,
			"algorithm":          string(cmkapi.KeyAlgorithmAES256),
			"region":             "us-west-2",
			"description":        "test key",
			"enabled":            true,
		}
		// Act
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: "keys",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, byokKey),
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups: []string{keyConfig.AdminGroup.IAMIdentifier},
				},
			},
		})
		// Assert
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "KEYSTORE_POOL_DRAINED", response.Error.Code)
	})
}

func TestKeyControllerGetKeysKeyID(t *testing.T) {
	db, sv, tenant := startAPIKeys(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	// Create a key in the database
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	r := sql.NewRepository(db)

	testutils.CreateTestEntities(ctx, t, r, key, keyConfig)

	tests := []struct {
		name           string
		keyID          string
		expectedStatus int
		expectedName   string
	}{
		{
			name:           "T200KeyGETByIdSuccess",
			keyID:          key.ID.String(),
			expectedStatus: http.StatusOK,
			expectedName:   key.Name,
		},
		{
			name:           "T201KeyGETByIdInvalidId",
			keyID:          "invalid-key-id",
			expectedStatus: http.StatusBadRequest,
			expectedName:   "",
		},
		{
			name:           "T202KeyGETByIdNotFound",
			keyID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			expectedName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/keys/" + tt.keyID,
				Tenant:   tenant,
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Groups: []string{keyConfig.AdminGroup.IAMIdentifier},
					},
				},
			})
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				k := testutils.GetJSONBody[cmkapi.Key](t, w)
				assert.Equal(t, tt.expectedName, k.Name)
			}
		})
	}

	t.Run("Should fail to get when no group permission", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/keys/" + key.ID.String(),
			Tenant:   tenant,
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups: []string{"some-other-group"},
				},
			},
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "KEY_CONFIGURATION_NOT_FOUND", response.Error.Code)
	})
}

func TestKeyControllerDeleteKeysKeyID(t *testing.T) {
	db, sv, tenant := startAPIKeys(t, testutils.KeyStorePlugin)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})

	keyConfigWSys := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfigWSys.ID)
	})
	pkey := testutils.NewKey(func(k *model.Key) {
		k.IsPrimary = true
		k.KeyConfigurationID = keyConfigWSys.ID
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		key,
		keyConfig,
		keystore,
		keystoreDefaultCert,
		keyConfigWSys,
		pkey,
		sys,
	)

	tests := []struct {
		name           string
		keyID          uuid.UUID
		expectedStatus int
	}{
		{
			name:           "T300KeyDELETEByIdSuccess",
			keyID:          key.ID,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "T301KeyDELETEByIdInvalidId",
			keyID:          uuid.New(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Should 400 on pkey delete with connected systems",
			keyID:          pkey.ID,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodDelete,
				Endpoint: "/keys/" + tt.keyID.String(),
				Tenant:   tenant,
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Groups: []string{keyConfig.AdminGroup.IAMIdentifier, keyConfigWSys.AdminGroup.IAMIdentifier},
					},
				},
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusNoContent {
				deletedKey := &model.Key{ID: tt.keyID}

				_, err := r.First(ctx, deletedKey, *repo.NewQuery())
				assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
			}
		})
	}

	t.Run("should not delete when no group permission", func(t *testing.T) {
		kc2 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		key2 := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = kc2.ID
		})

		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			kc2,
			key2,
		)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodDelete,
			Endpoint: "/keys/" + key2.ID.String(),
			Tenant:   tenant,
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups: []string{keyConfig.AdminGroup.IAMIdentifier, keyConfigWSys.AdminGroup.IAMIdentifier},
				},
			},
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "KEY_CONFIGURATION_NOT_FOUND", response.Error.Code)
	})
}

func TestKeyControllerUpdateKey(t *testing.T) {
	db, sv, tenant := startAPIKeys(t, testutils.KeyStorePlugin)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	regionEditable := "region1"
	regionNonEditable := "region2"

	cryptoData, err := json.Marshal(model.KeyAccessData{
		regionEditable:    map[string]any{},
		regionNonEditable: map[string]any{},
	})
	assert.NoError(t, err)

	kc := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	sysFailed := testutils.NewSystem(func(sys *model.System) {
		sys.KeyConfigurationID = ptr.PointTo(kc.ID)
		sys.Region = regionEditable
		sys.Status = cmkapi.SystemStatusFAILED
	})

	sys := testutils.NewSystem(func(sys *model.System) {
		sys.KeyConfigurationID = ptr.PointTo(kc.ID)
		sys.Region = regionNonEditable
		sys.Status = cmkapi.SystemStatusCONNECTED
	})

	key := testutils.NewKey(func(k *model.Key) {
		k.IsPrimary = true
		k.CryptoAccessData = cryptoData
		k.ManagementAccessData = json.RawMessage("{\"test\":\"test\"}")
		k.KeyConfigurationID = kc.ID
		k.Provider = providerTest
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		key,
		kc,
		keystore,
		keystoreDefaultCert,
		sysFailed,
		sys,
	)

	tests := []struct {
		name           string
		keyID          string
		input          cmkapi.KeyPatch
		expectedStatus int
		expectedName   string
		expectedDesc   string
	}{
		{
			name:  "T400KeyUPDATESuccess",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				Description: ptr.PointTo("updated description"),
				Name:        ptr.PointTo("updated-key"),
				Enabled:     ptr.PointTo(true),
			},
			expectedStatus: http.StatusOK,
			expectedName:   "updated-key",
			expectedDesc:   "updated description",
		},
		{
			name:  "T400KeyUPDATESuccessDisable",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				Description: ptr.PointTo("updated description"),
				Name:        ptr.PointTo("updated-key"),
				Enabled:     ptr.PointTo(false),
			},
			expectedStatus: http.StatusOK,
			expectedName:   "updated-key",
			expectedDesc:   "updated description",
		},
		{
			name:  "T400KeyUPDATESuccessEnable",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				Description: ptr.PointTo("updated description"),
				Name:        ptr.PointTo("updated-key"),
				Enabled:     ptr.PointTo(true),
			},
			expectedStatus: http.StatusOK,
			expectedName:   "updated-key",
			expectedDesc:   "updated description",
		},
		{
			name:  "T401KeyUPDATEInvalidId",
			keyID: "invalid-key-id",
			input: cmkapi.KeyPatch{
				Description: ptr.PointTo("updated description"),
				Name:        ptr.PointTo("updated-key"),
				Enabled:     ptr.PointTo(true),
			},
			expectedStatus: http.StatusBadRequest,
			expectedName:   "",
			expectedDesc:   "",
		},
		{
			name:  "T402KeyUPDATENotFound",
			keyID: uuid.New().String(),
			input: cmkapi.KeyPatch{
				Description: ptr.PointTo("updated description"),
				Name:        ptr.PointTo("updated-key"),
				Enabled:     ptr.PointTo(true),
			},
			expectedStatus: http.StatusNotFound,
			expectedName:   "",
			expectedDesc:   "",
		},
		{
			name:  "Should error on unmark primary key",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				IsPrimary: ptr.PointTo(false),
			},
			expectedStatus: http.StatusForbidden,
			expectedName:   "",
			expectedDesc:   "",
		},
		{
			name:  "Should code 403 on management role update",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				IsPrimary: ptr.PointTo(false),
				AccessDetails: &cmkapi.KeyAccessDetails{
					Management: &map[string]any{
						"a": "b",
					},
				},
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:  "Should code 403 on non editable crypto region",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				AccessDetails: &cmkapi.KeyAccessDetails{
					Crypto: &map[string]map[string]any{
						regionNonEditable: {
							"key": "value",
						},
					},
				},
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:  "Should 200 on valid crypto access data update",
			keyID: key.ID.String(),
			input: cmkapi.KeyPatch{
				Description: ptr.PointTo("updated description"),
				Name:        ptr.PointTo("updated-key"),
				Enabled:     ptr.PointTo(true),
				AccessDetails: &cmkapi.KeyAccessDetails{
					Crypto: &map[string]map[string]any{
						regionEditable: {
							"key": "value",
						},
						"new-region": {
							"key": "value",
						},
					},
				},
			},
			expectedStatus: http.StatusOK,
			expectedName:   "updated-key",
			expectedDesc:   "updated description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPatch,
				Endpoint: "/keys/" + tt.keyID,
				Tenant:   tenant,
				Body:     testutils.WithJSON(t, tt.input),
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Groups: []string{kc.AdminGroup.IAMIdentifier},
					},
				},
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.Key](t, w)
				assert.Equal(t, tt.expectedName, response.Name)
				assert.Equal(t, tt.expectedDesc, *response.Description)
			}
		})
	}

	t.Run("should not update when no group permission", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/keys/" + key.ID.String(),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, cmkapi.KeyPatch{Name: ptr.PointTo("new-name")}),
			AdditionalContext: map[any]any{
				constants.ClientData: &auth.ClientData{
					Groups: []string{"some-other-group"},
				},
			},
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "KEY_CONFIGURATION_NOT_FOUND", response.Error.Code)
	})
}

func TestKeyControllerGetImportParams(t *testing.T) {
	db, sv, tenant := startAPIKeys(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	// Create a BYOK key and import params in the database
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyType = string(cmkapi.KeyTypeBYOK)
		k.State = string(cmkapi.KeyStatePENDINGIMPORT)
	})

	importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
		ip.PublicKeyPEM = key.Name
		ip.KeyID = key.ID
	})

	byokEnabled := testutils.NewKey(func(k *model.Key) {
		k.KeyType = string(cmkapi.KeyTypeBYOK)
		k.State = string(cmkapi.KeyStateENABLED)
	})

	sysManagedKey := testutils.NewKey(func(_ *model.Key) {})

	hyokKey := testutils.NewKey(func(k *model.Key) {
		k.KeyType = string(cmkapi.KeyTypeHYOK)
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		key,
		byokEnabled,
		sysManagedKey,
		hyokKey,
		importParams,
		keystore,
		keystoreDefaultCert,
	)

	t.Run("GetImportParamsSuccess", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/keys/%s/importParams", key.ID.String()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.ImportParams](t, w)

		assert.Equal(t, key.Name, *response.PublicKey)
		assert.EqualValues(t, "CKM_RSA_AES_KEY_WRAP", response.WrappingAlgorithm.Name)
		assert.EqualValues(t, "SHA256", response.WrappingAlgorithm.HashFunction)
	})

	t.Run("GetImportParamsInvalidKeyTypeSYSTEM_MANAGED", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/keys/%s/importParams", sysManagedKey.ID.String()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)

		assert.Equal(t, "INVALID_ACTION_FOR_KEY_TYPE", response.Error.Code)
		assert.Equal(
			t, "The action cannot be performed for the key type. Only BYOK keys can get import parameters.",
			response.Error.Message)
	})

	t.Run("GetImportParamsInvalidKeyTypeHYOK", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/keys/%s/importParams", hyokKey.ID.String()),
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)

		assert.Equal(t, "INVALID_ACTION_FOR_KEY_TYPE", response.Error.Code)
		assert.Equal(
			t, "The action cannot be performed for the key type. Only BYOK keys can get import parameters.",
			response.Error.Message)
	})

	t.Run("GetImportParamsInvalidKeyState", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/keys/%s/importParams", byokEnabled.ID.String()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)

		assert.Equal(t, "INVALID_KEY_STATE", response.Error.Code)
		assert.Equal(t, "Key must be in PENDING_IMPORT state to get import parameters.",
			response.Error.Message)
	})

	t.Run("GetImportParamsInvalidId", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/keys/a/importParams",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetImportParamsNotFound", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/keys/%s/importParams", uuid.New()),
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GetImportParamsDBError", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/keys/%s/importParams", key.ID.String()),
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestKeyControllerImportKeyMaterial(t *testing.T) {
	db, sv, tenant := startAPIKeys(t, testutils.KeyStorePlugin)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyType = string(cmkapi.KeyTypeBYOK)
		k.State = string(cmkapi.KeyStatePENDINGIMPORT)
		k.NativeID = ptr.PointTo("arn:aws:kms:us-west-2:123456789012:key/12345678-90ab-cdef-1234-567890abcdef")
	})

	paramsJSON, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	assert.NoError(t, err)

	importParams := model.ImportParams{
		KeyID:              key.ID,
		PublicKeyPEM:       "test-public-key",
		WrappingAlg:        "CKM_RSA_AES_KEY_WRAP",
		HashFunction:       "SHA256",
		ProviderParameters: paramsJSON,
	}

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		key,
		&importParams,
		keystore,
		keystoreDefaultCert,
	)

	t.Run("ImportKeyMaterialSuccess", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", key.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("ImportKeyMaterialFailedEmptyKeyMaterial", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", key.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: "",
			}),
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)

		assert.Equal(t, "IMPORT_KEY_MATERIAL", response.Error.Code)
		assert.Equal(t, "Key material cannot be empty.", response.Error.Message)
	})

	t.Run("ImportKeyMaterialFailedNonBase64KeyMaterial", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", key.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: "non-base64-key-material",
			}),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "IMPORT_KEY_MATERIAL", response.Error.Code)
		assert.Equal(t, "Key material must be base64 encoded.", response.Error.Message)
	})

	t.Run("ImportKeyMaterialFailedNoImportParams", func(t *testing.T) {
		byokNoImportParams := testutils.NewKey(func(k *model.Key) {
			k.Name = "byok-no-import-params"
			k.KeyType = string(cmkapi.KeyTypeBYOK)
			k.State = string(cmkapi.KeyStatePENDINGIMPORT)
			k.NativeID = ptr.PointTo("arn:aws:kms:us-west-2:123456789012:key/12345678-90ab-cdef-6789-567890abcdef")
		})
		testutils.CreateTestEntities(ctx, t, r, byokNoImportParams)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", byokNoImportParams.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "IMPORT_KEY_MATERIAL", response.Error.Code)
		assert.Equal(t, "Import parameters missing or expired. Please request new import parameters.", response.Error.Message)
	})

	t.Run("ImportKeyMaterialFailedInvalidKeyTypeSYSTEM_MANAGED", func(t *testing.T) {
		// Prepare
		sysManagedKey := testutils.NewKey(func(k *model.Key) {
			k.Name = "sys-managed-key"
		})
		testutils.CreateTestEntities(ctx, t, r, sysManagedKey)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", sysManagedKey.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "INVALID_ACTION_FOR_KEY_TYPE", response.Error.Code)
		assert.Equal(t,
			"The action cannot be performed for the key type. Only BYOK keys can import key material.",
			response.Error.Message,
		)
	})

	t.Run("ImportKeyMaterialFailedInvalidKeyTypeHYOK", func(t *testing.T) {
		hyokKey := testutils.NewKey(func(k *model.Key) {
			k.Name = "hyok-key"
			k.KeyType = string(cmkapi.KeyTypeHYOK)
		})

		params := model.ImportParams{
			KeyID:              key.ID,
			PublicKeyPEM:       "test-public-key",
			WrappingAlg:        "CKM_RSA_AES_KEY_WRAP",
			HashFunction:       "SHA256",
			ProviderParameters: paramsJSON,
			Expires:            ptr.PointTo(time.Now().Add(1 * time.Hour)),
		}

		testutils.CreateTestEntities(ctx, t, r, hyokKey, &params)
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", hyokKey.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "INVALID_ACTION_FOR_KEY_TYPE", response.Error.Code)
		assert.Equal(
			t,
			"The action cannot be performed for the key type. Only BYOK keys can import key material.",
			response.Error.Message,
		)
	})

	t.Run("ImportKeyMaterialFailedInvalidKeyState", func(t *testing.T) {
		// Prepare
		byokEnabled := testutils.NewKey(func(k *model.Key) {
			k.KeyType = string(cmkapi.KeyTypeBYOK)
			k.State = string(cmkapi.KeyStateENABLED)
		})

		testutils.CreateTestEntities(ctx, t, r, byokEnabled)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", byokEnabled.ID.String()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "INVALID_KEY_STATE", response.Error.Code)
		assert.Equal(t, "Key must be in PENDING_IMPORT state to import key material.", response.Error.Message)
	})

	t.Run("ImportKeyMaterialFailedInvalidKeyID", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", "error"),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "PARAMS_ERROR", response.Error.Code)
		assert.Contains(t, response.Error.Message, "Invalid format for parameter keyID")
	})

	t.Run("ImportKeyMaterialFailedNotFound", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", uuid.NewString()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		assert.Equal(t, http.StatusNotFound, w.Code)

		response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, "KEY_ID", response.Error.Code)
		assert.Equal(t, "Key by KeyID not found", response.Error.Message)
	})

	t.Run("ImportKeyMaterialFailedDBError", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: fmt.Sprintf("/keys/%s/importKeyMaterial", uuid.NewString()),
			Tenant:   tenant,
			Body: testutils.WithJSON(t, cmkapi.KeyImport{
				WrappedKeyMaterial: base64.StdEncoding.EncodeToString([]byte("test-wrapped-key-material")),
			}),
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
