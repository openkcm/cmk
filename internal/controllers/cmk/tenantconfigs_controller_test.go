package cmk_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/crypto"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// startAPIServerTenantConfig starts the API server for keys and returns a pointer to the database
func startAPIServerTenantConfig(t *testing.T, cfg testutils.TestAPIServerConfig) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	cfg.Config.Database = dbCfg

	return db, testutils.NewAPIServer(t, db, cfg), tenants[0]
}

func TestAPIController_GetTenantKeystores(t *testing.T) {
	db, sv, tenant := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	}, testutils.WithAuthClientDataKC(authClient))
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should 200 on get keystores", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/tenantConfigurations/keystores",
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// getWorkflowConfig is a helper function to retrieve workflow configuration via API
func getWorkflowConfig(t *testing.T, sv cmkapi.ServeMux,
	tenant string, authClient testutils.AuthClientData,
) cmkapi.TenantWorkflowConfiguration {
	t.Helper()

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodGet,
		Endpoint:          "/tenantConfigurations/workflow",
		Tenant:            tenant,
		AdditionalContext: authClient.GetClientMap(),
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response cmkapi.TenantWorkflowConfiguration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	return response
}

func TestAPIController_GetTenantWorkflowConfiguration(t *testing.T) {
	t.Run("Should 200 getting workflow config", func(t *testing.T) {
		db, sv, tenant := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Setup: Create a workflow config
		setupWorkflowConfig(t, r, ctx)

		// Test
		response := getWorkflowConfig(t, sv, tenant, authClient)

		assert.NotNil(t, response.Enabled)
		assert.True(t, *response.Enabled)
		assert.NotNil(t, response.MinimumApprovals)
		assert.Equal(t, 3, *response.MinimumApprovals)
		assert.NotNil(t, response.RetentionPeriodDays)
		assert.Equal(t, 45, *response.RetentionPeriodDays)
	})

	t.Run("Should 200 getting default workflow config when none exists", func(t *testing.T) {
		db, sv, tenant := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		response := getWorkflowConfig(t, sv, tenant, authClient)

		assert.NotNil(t, response.Enabled)
		assert.NotNil(t, response.MinimumApprovals)
		assert.NotNil(t, response.RetentionPeriodDays)
	})
}

func setupWorkflowConfig(t *testing.T, r *sql.ResourceRepository, ctx context.Context) {
	t.Helper()

	workflowConfig := testutils.NewDefaultWorkflowConfig(true)
	workflowConfig.MinimumApprovals = 3
	workflowConfig.RetentionPeriodDays = 45

	configJSON, err := json.Marshal(workflowConfig)
	require.NoError(t, err)

	tenantConfig := &model.TenantConfig{
		Key:   constants.WorkflowConfigKey,
		Value: configJSON,
	}
	err = r.Set(ctx, tenantConfig)
	require.NoError(t, err)
}

func TestAPIController_UpdateTenantWorkflowConfiguration(t *testing.T) {
	t.Run("Should 200 updating workflow configuration for tenant admin", func(t *testing.T) {
		db, sv, tenant := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Setup: Create initial workflow config
		workflowConfig := testutils.NewDefaultWorkflowConfig(false)
		configJSON, err := json.Marshal(workflowConfig)
		require.NoError(t, err)

		tenantConfig := &model.TenantConfig{
			Key:   constants.WorkflowConfigKey,
			Value: configJSON,
		}
		err = r.Set(ctx, tenantConfig)
		require.NoError(t, err)

		// Test: Update config
		updateRequest := cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals:    ptr.PointTo(5),
			RetentionPeriodDays: ptr.PointTo(60),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodPatch,
			Endpoint:          "/tenantConfigurations/workflow",
			Tenant:            tenant,
			Body:              testutils.WithJSON(t, updateRequest),
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusOK, w.Code)

		var response cmkapi.TenantWorkflowConfiguration
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotNil(t, response.MinimumApprovals)
		assert.Equal(t, 5, *response.MinimumApprovals)
		assert.NotNil(t, response.RetentionPeriodDays)
		assert.Equal(t, 60, *response.RetentionPeriodDays)
		assert.NotNil(t, response.Enabled)
		assert.False(t, *response.Enabled) // Should remain unchanged
	})

	t.Run("Should 400 with invalid retention period", func(t *testing.T) {
		db, sv, tenant := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Setup: Create initial workflow config
		setupDefaultWorkflowConfig(t, r, ctx)

		// Test: Update with invalid retention period
		updateRequest := cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: ptr.PointTo(1), // Less than minimum of 2
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodPatch,
			Endpoint:          "/tenantConfigurations/workflow",
			Tenant:            tenant,
			Body:              testutils.WithJSON(t, updateRequest),
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
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

			db, sv, tenant := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{
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
			r := sql.NewRepository(db)
			ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

			key1 := testutils.NewKey(func(_ *model.Key) {})

			authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

			keyconfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
				c.PrimaryKeyID = &key1.ID
			}, testutils.WithAuthClientDataKC(authClient))

			testutils.CreateTestEntities(ctx, t, r, key1, keyconfig)

			if tt.setupFunc != nil {
				tt.setupFunc(t, db, tenant)
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/tenantConfigurations/certificates",
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

func setupDefaultWorkflowConfig(t *testing.T, r *sql.ResourceRepository, ctx context.Context) {
	t.Helper()

	workflowConfig := testutils.NewDefaultWorkflowConfig(false)
	configJSON, err := json.Marshal(workflowConfig)
	require.NoError(t, err)

	tenantConfig := &model.TenantConfig{
		Key:   constants.WorkflowConfigKey,
		Value: configJSON,
	}
	err = r.Set(ctx, tenantConfig)
	require.NoError(t, err)
}
