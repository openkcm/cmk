//go:build !unit

package cmk_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/crypto"
	"github.com/openkcm/cmk/utils/ptr"
)

// startAPIKeyConfig starts the API server for key configurations and returns a pointer to the database
func startAPIKeyConfig(t *testing.T, cfg testutils.TestAPIServerConfig) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Key{},
			&model.KeyVersion{},
			&model.KeyConfiguration{},
			&model.Tenant{},
			&model.TenantConfig{},
			&model.System{},
			&model.Certificate{},
		},
	})

	return db, testutils.NewAPIServer(t, db, cfg), tenants[0]
}

func TestKeyConfigurationGetConfiguration(t *testing.T) {
	db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should get keyConfig", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations/" + keyConfig.ID.String(),
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyConfiguration](t, w)

		assert.Equal(t, keyConfig.PrimaryKeyID, response.PrimaryKeyID)
		assert.Equal(t, keyConfig.Name, response.Name)
		assert.True(t, *response.CanConnectSystems)
	})
}

func TestKeyConfigurationGetConfigurationsWithGroups(t *testing.T) {
	db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	groupID := uuid.New()
	group := testutils.NewGroup(func(g *model.Group) {
		g.ID = groupID
	})

	repo := sql.NewRepository(db)
	err := repo.Create(testutils.CreateCtxWithTenant(tenant), group)
	assert.NoError(t, err)

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
		k.AdminGroupID = groupID
		k.AdminGroup = *group
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should get keyConfig", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations?expandGroup=true",
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.KeyConfigurationList](t, w)

		assert.Equal(t, keyConfig.PrimaryKeyID, response.Value[0].PrimaryKeyID)
		assert.Equal(t, keyConfig.Name, response.Value[0].Name)
		assert.Equal(t, keyConfig.AdminGroupID, response.Value[0].AdminGroupID)
		assert.Equal(t, groupID, response.Value[0].AdminGroupID)

		assert.Equal(t, groupID, *response.Value[0].AdminGroup.Id)
		assert.Equal(t, group.Name, response.Value[0].AdminGroup.Name)
		assert.Equal(t, string(group.Role), string(response.Value[0].AdminGroup.Role))

		assert.True(t, *response.Value[0].CanConnectSystems)
	})
}

func TestKeyconfigurationControllerGetKeyconfigurationsPagination(t *testing.T) {
	db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	for range totalRecordCount {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig)
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
	db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	group := testutils.NewGroup(func(_ *model.Group) {})

	testutils.CreateTestEntities(ctx, t, r, group)

	type testCase struct {
		name           string
		input          cmkapi.KeyConfiguration
		expectedStatus int
		expectedCode   string
		expectedBody   string
	}

	tests := []testCase{
		{
			name: "KeyConfigPOST_Success",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: group.ID,
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   "test-config",
		},
		{
			name: "KeyConfigPOST_MissingName",
			input: cmkapi.KeyConfiguration{
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: group.ID,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
		},
		{
			name: "KeyConfigPOST_EmptyName",
			input: cmkapi.KeyConfiguration{
				Name:         "",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: group.ID,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
		},
		{
			name: "KeyConfigPOST_MissingAdminGroupID",
			input: cmkapi.KeyConfiguration{
				Name:        "",
				Description: ptr.PointTo("test-config"),
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
		},
		{
			name: "KeyConfigPOST_InvalidAdminGroupID",
			input: cmkapi.KeyConfiguration{
				Name:         "",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: uuid.New(),
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
		},
		{
			name: "KeyConfigPOST_DuplicateName",
			input: cmkapi.KeyConfiguration{
				Name:         "test-config",
				Description:  ptr.PointTo("test-config"),
				AdminGroupID: group.ID,
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "UNIQUE_ERROR",
			expectedBody:   "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPost,
				Endpoint: "/keyConfigurations",
				Tenant:   tenant,
				Body:     testutils.WithJSON(t, tt.input),
			})
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)

			if tt.expectedCode != "" {
				response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
				assert.Equal(t, tt.expectedCode, response.Error.Code)
			}
		})
	}
}

func TestKeyConfigurationController_UpdateByID(t *testing.T) {
	db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)
	newAdminGroupID := uuid.New()

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	existingKeyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.Name = "existing-config"
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, existingKeyConfig)

	type testCase struct {
		name           string
		configID       string
		inputJSON      string
		expectedStatus int
		expectedBody   string
		expectedCode   string
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}

	tests := []testCase{
		{
			name:     "KeyConfigPATCH_Success",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-config",
                "description": "updated description"
            }`,
			expectedStatus: http.StatusOK,
			expectedBody:   "updated-config",
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
			expectedStatus: http.StatusOK,
			expectedBody:   "updated-name-only",
		},
		{
			name:     "KeyConfigPATCH_DescriptionOnly",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "description": "updated description only"
            }`,
			expectedStatus: http.StatusOK,
			expectedBody:   "updated description only",
		},
		{
			name:     "KeyConfigPATCH_EmptyName",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": ""
            }`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
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

				config := &model.KeyConfiguration{ID: keyConfig.ID}
				_, err := r.First(ctx, config, *repo.NewQuery())
				assert.NoError(t, err)
			},
		},
		{
			name:     "KeyConfigPATCH_NameConflict",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "existing-config"
            }`,
			expectedStatus: http.StatusConflict,
			expectedCode:   "UNIQUE_ERROR",
			expectedBody:   "error",
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
			expectedStatus: http.StatusNotFound,
			expectedBody:   "error",
		},
		{
			name:     "KeyConfigPATCH_InvalidJSON",
			configID: keyConfig.ID.String(),
			inputJSON: `{
                "name": "updated-config",
                invalid json
            }`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPatch,
				Endpoint: "/keyConfigurations/" + tt.configID,
				Tenant:   tenant,
				Body:     testutils.WithString(t, tt.inputJSON),
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
	db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	keyConfigWithSystems := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfigWithSystems.ID)
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, keyConfigWithSystems, sys)

	type testCase struct {
		name           string
		configID       string
		expectedStatus int
	}

	tests := []testCase{
		{
			name:           "DeleteKeyConfig_Success",
			configID:       keyConfig.ID.String(),
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "DeleteKeyConfig_InvalidID",
			configID:       "invalid-id",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "DeleteKeyConfig_NoConfigStillSuccess",
			configID:       uuid.New().String(),
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "Should 400 on deletion with connected systems",
			configID:       keyConfigWithSystems.ID.String(),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodDelete,
				Endpoint: "/keyConfigurations/" + tt.configID,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestAPIController_GetCertificates(t *testing.T) {
	key1 := testutils.NewKey(func(_ *model.Key) {})

	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.PrimaryKeyID = &key1.ID
	})

	tests := []struct {
		name                string
		expectedStatus      int
		expectedError       string
		setupFunc           func(t *testing.T, db *multitenancy.DB, tenant string)
		expectedRecordCount int
		expectedRootCA      string
		expectedSubject     string
		expectedType        string
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
			expectedStatus:      http.StatusInternalServerError,
			expectedError:       "GET_CLIENT_CERTIFICATES",
			expectedRecordCount: 0,
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

			db, sv, tenant := startAPIKeyConfig(t, testutils.TestAPIServerConfig{
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

			if tt.setupFunc != nil {
				tt.setupFunc(t, db, tenant)
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: fmt.Sprintf("/keyConfigurations/%s/certificates", keyConfig.ID.String()),
				Tenant:   tenant,
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
