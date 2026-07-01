package cmk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

// startAPIServerTenantConfig starts the API server for keys and returns a pointer to the database
func startAPIServerTenantConfig(t *testing.T, cfg testutils.TestAPIServerConfig) (*multitenancy.DB, cmkapi.ServeMux, string, *testutils.TestSigningKeyStorage) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	cfg.Config.Database = dbCfg
	cfg.EnableBusinessUserDataMW = true

	keyStorage := testutils.NewTestSigningKeyStorage(t)
	cfg.SigningKeyStorage = keyStorage

	return db, testutils.NewAPIServer(t, db, cfg), tenants[0], keyStorage
}

func TestAPIController_GetTenantKeystores(t *testing.T) {
	db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	}, testutils.WithAuthBusinessUserDataKC(authClient))
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	businessUserData := &auth.ClientData{
		Identifier: authClient.Identifier,
		Groups:     []string{authClient.Group.IAMIdentifier},
	}

	privateKey, ok := keyStorage.GetPrivateKey(0)
	assert.True(t, ok, "test key should exist")
	headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

	t.Run("Should 200 on get keystores", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/tenantConfigurations/keystores",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// getWorkflowConfig is a helper function to retrieve workflow configuration via API
func getWorkflowConfig(t *testing.T, sv cmkapi.ServeMux, tenant string,
	headers http.Header,
) cmkapi.TenantWorkflowConfiguration {
	t.Helper()

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodGet,
		Endpoint: "/tenantConfigurations/workflow",
		Tenant:   tenant,
		Headers:  headers,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response cmkapi.TenantWorkflowConfiguration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	return response
}

func TestAPIController_GetTenantWorkflowConfiguration(t *testing.T) {
	t.Run("Should 200 getting workflow config", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Setup: Create a workflow config
		setupWorkflowConfig(t, r, ctx)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}

		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		// Test
		response := getWorkflowConfig(t, sv, tenant, headers)
		assert.NotNil(t, response.MinimumApprovals)
		assert.Equal(t, 3, *response.MinimumApprovals)
		assert.NotNil(t, response.RetentionPeriodDays)
		assert.Equal(t, 45, *response.RetentionPeriodDays)
	})

	t.Run("Should 200 getting default workflow config when none exists", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}

		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		response := getWorkflowConfig(t, sv, tenant, headers)

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
		Value: string(configJSON),
	}
	err = r.Set(ctx, tenantConfig)
	require.NoError(t, err)
}

func TestAPIController_UpdateTenantWorkflowConfiguration(t *testing.T) {
	t.Run("Should 200 updating workflow configuration for tenant admin", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Setup: Create initial workflow config
		workflowConfig := testutils.NewDefaultWorkflowConfig(false)
		configJSON, err := json.Marshal(workflowConfig)
		require.NoError(t, err)

		tenantConfig := &model.TenantConfig{
			Key:   constants.WorkflowConfigKey,
			Value: string(configJSON),
		}
		err = r.Set(ctx, tenantConfig)
		require.NoError(t, err)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}

		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		// Test: Update config
		updateRequest := cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals:    ptr.PointTo(5),
			RetentionPeriodDays: ptr.PointTo(60),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateRequest),
			Headers:  headers,
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
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Setup: Create initial workflow config
		setupDefaultWorkflowConfig(t, r, ctx)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}

		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		// Test: Update with invalid retention period
		updateRequest := cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: ptr.PointTo(29), // Less than minimum of 30
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateRequest),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should 400 INVALID_SETTING when defaultExpiryPeriodDays exceeds maxExpiryPeriodDays", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())
		setupDefaultWorkflowConfig(t, r, ctx)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}
		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		updateRequest := cmkapi.TenantWorkflowConfiguration{
			DefaultExpiryPeriodDays: ptr.PointTo(20),
			MaxExpiryPeriodDays:     ptr.PointTo(10),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateRequest),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp cmkapi.ErrorMessage
		err := json.Unmarshal(w.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_SETTING", errResp.Error.Code)
		assert.NotNil(t, errResp.Error.Context)
		assert.Equal(t, "defaultExpiryPeriodDays", (*errResp.Error.Context)["setting"])
	})

	t.Run("Should 400 INVALID_SETTING when minimumApprovals is less than 2", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())
		setupDefaultWorkflowConfig(t, r, ctx)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}
		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		updateRequest := cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals: ptr.PointTo(1),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateRequest),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp cmkapi.ErrorMessage
		err := json.Unmarshal(w.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_SETTING", errResp.Error.Code)
		assert.NotNil(t, errResp.Error.Context)
		assert.Equal(t, "minimumApprovals", (*errResp.Error.Context)["setting"])
	})

	t.Run("Should 400 INVALID_SETTING when retentionPeriodDays is less than 30", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())
		setupDefaultWorkflowConfig(t, r, ctx)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}
		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		updateRequest := cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: ptr.PointTo(29),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateRequest),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp cmkapi.ErrorMessage
		err := json.Unmarshal(w.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_SETTING", errResp.Error.Code)
		assert.NotNil(t, errResp.Error.Context)
		assert.Equal(t, "retentionPeriodDays", (*errResp.Error.Context)["setting"])
	})

	t.Run("Should 400 INVALID_SETTING when non-TEST tenant tries to disable workflow", func(t *testing.T) {
		db, sv, tenant, keyStorage := startAPIServerTenantConfig(t, testutils.TestAPIServerConfig{})
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

		// Store config with enabled=true so changing to false triggers role validation
		enabledConfig := testutils.NewDefaultWorkflowConfig(true)
		configJSON, err := json.Marshal(enabledConfig)
		require.NoError(t, err)
		err = r.Set(ctx, &model.TenantConfig{Key: constants.WorkflowConfigKey, Value: string(configJSON)})
		require.NoError(t, err)

		businessUserData := &auth.ClientData{
			Identifier: authClient.Identifier,
			Groups:     []string{authClient.Group.IAMIdentifier},
		}
		privateKey, ok := keyStorage.GetPrivateKey(0)
		assert.True(t, ok, "test key should exist")
		headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

		updateRequest := cmkapi.TenantWorkflowConfiguration{
			Enabled: ptr.PointTo(false),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateRequest),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp cmkapi.ErrorMessage
		err = json.Unmarshal(w.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_SETTING", errResp.Error.Code)
		assert.NotNil(t, errResp.Error.Context)
		assert.Equal(t, "enabled", (*errResp.Error.Context)["setting"])
	})
}

func setupDefaultWorkflowConfig(t *testing.T, r *sql.ResourceRepository, ctx context.Context) {
	t.Helper()

	workflowConfig := testutils.NewDefaultWorkflowConfig(false)
	configJSON, err := json.Marshal(workflowConfig)
	require.NoError(t, err)

	tenantConfig := &model.TenantConfig{
		Key:   constants.WorkflowConfigKey,
		Value: string(configJSON),
	}
	err = r.Set(ctx, tenantConfig)
	require.NoError(t, err)
}
