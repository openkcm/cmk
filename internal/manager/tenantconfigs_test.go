package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

var ErrForced = errors.New("forced")

func SetupTenantConfigManager(t *testing.T, plugins []testutils.MockPlugin) (*manager.TenantConfigManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	dbRepository := sql.NewRepository(db)
	cfg := config.Config{Plugins: testutils.SetupMockPlugins(plugins...)}
	ctlg, err := catalog.New(t.Context(), &cfg)
	assert.NoError(t, err)

	tenantManager := manager.NewTenantConfigManager(dbRepository, ctlg, nil)

	return tenantManager, db, tenants[0]
}

// SetupTenantConfigManagerWithRole creates a test tenant with a specific role
func SetupTenantConfigManagerWithRole(t *testing.T, role string, plugins []testutils.MockPlugin) (*manager.TenantConfigManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithTenantRole(model.TenantRole(role)))

	dbRepository := sql.NewRepository(db)
	cfg := config.Config{Plugins: testutils.SetupMockPlugins(plugins...)}
	ctlg, err := catalog.New(t.Context(), &cfg)
	assert.NoError(t, err)

	tenantManager := manager.NewTenantConfigManager(dbRepository, ctlg, nil)

	return tenantManager, db, tenants[0]
}

func TestNewTenantConfigManager(t *testing.T) {
	m, _, _ := SetupTenantConfigManager(t, nil)

	assert.NotNil(t, m)
}

// TestGetDefaultKeystore tests the GetDefaultKeystore method
func TestGetDefaultKeystore(t *testing.T) {
	t.Run("DefaultKeystore tenant config not exists, get from pool", func(t *testing.T) {
		// Arrange
		configManager, db, tenant := SetupTenantConfigManager(t, nil)
		// Add a keystore configuration to the pool
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)
		testutils.CreateTestEntities(ctx, t, r, ksConfig)

		// Act
		keystore, err := configManager.GetDefaultKeystoreConfig(testutils.CreateCtxWithTenant(tenant))

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, keystore)
		assert.NotEmpty(t, keystore.LocalityID)
		assert.NotEmpty(t, keystore.CommonName)
		assert.NotEmpty(t, keystore.ManagementAccessData)
	})

	t.Run("Config Exists", func(t *testing.T) {
		// Arrange
		configManager, db, tenant := SetupTenantConfigManager(t, nil)

		tenantConfigRepo := sql.NewRepository(db)
		ksConfigJSON, err := json.Marshal(&model.KeystoreConfig{
			LocalityID: testutils.TestLocalityID,
			CommonName: testutils.TestDefaultKeystoreCommonName,
			ManagementAccessData: map[string]any{
				"roleArn":        testutils.TestRoleArn,
				"trustAnchorArn": testutils.TestTrustAnchorArn,
				"profileArn":     testutils.TestProfileArn,
			},
		})
		assert.NoError(t, err)

		conf := &model.TenantConfig{
			Key:   constants.DefaultKeyStore,
			Value: ksConfigJSON,
		}

		err = tenantConfigRepo.Set(testutils.CreateCtxWithTenant(tenant), conf)
		assert.NoError(t, err)

		// Act
		keystore, err := configManager.GetDefaultKeystoreConfig(testutils.CreateCtxWithTenant(tenant))

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, testutils.TestLocalityID, keystore.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, keystore.CommonName)
		assert.Equal(t, testutils.TestRoleArn, keystore.ManagementAccessData["roleArn"])
		assert.Equal(t, testutils.TestTrustAnchorArn, keystore.ManagementAccessData["trustAnchorArn"])
		assert.Equal(t, testutils.TestProfileArn, keystore.ManagementAccessData["profileArn"])
	})
}

func TestSetDefaultKeystore(t *testing.T) {
	t.Run("DefaultKeystore tenant config not exists, set default keystore", func(t *testing.T) {
		// Arrange
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)

		// Act
		err := configManager.SetDefaultKeystore(
			ctx,
			testutils.NewKeystoreConfig(func(_ *model.KeystoreConfig) {}),
		)

		// Assert
		assert.NoError(t, err)
		keystore, err := configManager.GetDefaultKeystoreConfig(ctx)
		assert.NoError(t, err)

		assert.Equal(t, testutils.TestLocalityID, keystore.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, keystore.CommonName)
		assert.Equal(t, testutils.TestRoleArn, keystore.ManagementAccessData["roleArn"])
		assert.Equal(t, testutils.TestTrustAnchorArn, keystore.ManagementAccessData["trustAnchorArn"])
		assert.Equal(t, testutils.TestProfileArn, keystore.ManagementAccessData["profileArn"])
	})

	t.Run("Update existing default keystore config", func(t *testing.T) {
		// Arrange
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)
		err := configManager.SetDefaultKeystore(
			ctx,
			testutils.NewKeystoreConfig(func(_ *model.KeystoreConfig) {}),
		)
		assert.NoError(t, err)

		newLocalityID := uuid.NewString()
		newRoleArn := "arn:aws:iam::123456789012:role/ExampleRoleUpdated"
		newTrustAnchorID := "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/" + uuid.NewString()
		newProfileArn := "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/" + uuid.NewString()

		// Act
		err = configManager.SetDefaultKeystore(ctx, testutils.NewKeystoreConfig(func(kc *model.KeystoreConfig) {
			kc.LocalityID = newLocalityID
			kc.CommonName = testutils.TestDefaultKeystoreCommonName
			kc.ManagementAccessData = map[string]any{
				"roleArn":        newRoleArn,
				"trustAnchorArn": newTrustAnchorID,
				"profileArn":     newProfileArn,
			}
		}))

		// Assert
		assert.NoError(t, err)
		keystore, err := configManager.GetDefaultKeystoreConfig(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, keystore)

		assert.Equal(t, newLocalityID, keystore.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, keystore.CommonName)
		assert.Equal(t, newRoleArn, keystore.ManagementAccessData["roleArn"])
		assert.Equal(t, newTrustAnchorID, keystore.ManagementAccessData["trustAnchorArn"])
		assert.Equal(t, newProfileArn, keystore.ManagementAccessData["profileArn"])
	})
}

func TestGetTenantConfigsHyokKeystore(t *testing.T) {
	tests := []struct {
		name           string
		expectedOutput []string
		enabledPlugins bool
	}{
		{
			name:           "Success - One HYOK provider",
			expectedOutput: []string{"TEST"},
			enabledPlugins: true,
		},
		{
			name:           "Success - No HYOK providers",
			expectedOutput: []string{},
			enabledPlugins: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{}
			if tt.enabledPlugins {
				cfg.Plugins = testutils.SetupMockPlugins(testutils.KeyStorePlugin)
			}

			ctlg, err := catalog.New(t.Context(), &cfg)
			assert.NoError(t, err)

			mgr := manager.NewTenantConfigManager(nil, ctlg, nil)

			result := mgr.GetTenantConfigsHyokKeystore()
			assert.ElementsMatch(t, tt.expectedOutput, result.Provider)
		})
	}
}

func TestGetTenantsKeystore(t *testing.T) {
	t.Run("Should get tenant keystores with hyok", func(t *testing.T) {
		m, _, _ := SetupTenantConfigManager(t, []testutils.MockPlugin{testutils.KeyStorePlugin})
		res, err := m.GetTenantsKeystores()
		assert.NoError(t, err)
		assert.NotEmpty(t, res.HYOK)
	})

	t.Run("Should get tenant keystores with no hyok providers", func(t *testing.T) {
		m, _, _ := SetupTenantConfigManager(t, nil)
		res, err := m.GetTenantsKeystores()
		assert.NoError(t, err)
		assert.Empty(t, res.HYOK)
	})
}

func TestUpdateWorkflowConfig(t *testing.T) {
	// Helper to setup config for a tenant
	setupConfig := func(t *testing.T,
		mgr *manager.TenantConfigManager, ctx context.Context, cfg *model.WorkflowConfig) {
		t.Helper()
		_, err := mgr.SetWorkflowConfig(ctx, cfg)
		assert.NoError(t, err)
	}

	t.Run("Should update workflow config with partial update", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals: ptr.PointTo(3),
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Enabled)
		assert.Equal(t, 3, result.MinimumApprovals)
		assert.Equal(t, 30, result.RetentionPeriodDays)
	})

	t.Run("Should update multiple fields at once", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals:    ptr.PointTo(3),
			RetentionPeriodDays: ptr.PointTo(60),
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Enabled)
		assert.Equal(t, 3, result.MinimumApprovals)
		assert.Equal(t, 60, result.RetentionPeriodDays)
		assert.Equal(t, 7, result.DefaultExpiryPeriodDays)
	})

	t.Run("Should fail when retention period is less than minimum", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: ptr.PointTo(1),
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, manager.ErrRetentionLessThanMinimum)
	})

	t.Run("Should create default config when updating non-existent config", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			Enabled: ptr.PointTo(true),
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Enabled)
	})

	t.Run("Should handle nil update gracefully", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Enabled)
		assert.Equal(t, 2, result.MinimumApprovals)
		assert.Equal(t, 30, result.RetentionPeriodDays)
	})

	t.Run("Role-based enable/disable validation", func(t *testing.T) {
		tests := []struct {
			name          string
			role          string
			initialState  bool
			targetState   bool
			shouldSucceed bool
		}{
			{"ROLE_LIVE cannot disable", "ROLE_LIVE", true, false, false},
			{"ROLE_TEST can enable", "ROLE_TEST", false, true, true},
			{"ROLE_TEST can disable", "ROLE_TEST", true, false, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var configManager *manager.TenantConfigManager
				var tenant string

				if tt.role == tenantpb.Role_ROLE_TEST.String() {
					configManager, _, tenant = SetupTenantConfigManagerWithRole(t, tt.role, nil)
				} else {
					configManager, _, tenant = SetupTenantConfigManager(t, nil)
				}

				ctx := testutils.CreateCtxWithTenant(tenant)
				setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(tt.initialState))

				result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
					Enabled: ptr.PointTo(tt.targetState),
				})

				if tt.shouldSucceed {
					assert.NoError(t, err)
					assert.NotNil(t, result)
					assert.Equal(t, tt.targetState, result.Enabled)
				} else {
					assert.Error(t, err)
					assert.Nil(t, result)
					assert.ErrorIs(t, err, manager.ErrWorkflowEnableDisableNotAllowed)
				}
			})
		}

		t.Run("ROLE_LIVE can update other fields without changing Enabled", func(t *testing.T) {
			configManager, _, tenant := SetupTenantConfigManager(t, nil)
			ctx := testutils.CreateCtxWithTenant(tenant)
			setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

			result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
				MinimumApprovals:    ptr.PointTo(5),
				RetentionPeriodDays: ptr.PointTo(90),
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Enabled)
			assert.Equal(t, 5, result.MinimumApprovals)
			assert.Equal(t, 90, result.RetentionPeriodDays)
		})

		t.Run("ROLE_TEST can update Enabled with other fields simultaneously", func(t *testing.T) {
			configManager, _, tenant := SetupTenantConfigManagerWithRole(t,
				tenantpb.Role_ROLE_TEST.String(), nil)
			ctx := testutils.CreateCtxWithTenant(tenant)
			setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(false))

			result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
				Enabled:             ptr.PointTo(true),
				MinimumApprovals:    ptr.PointTo(4),
				RetentionPeriodDays: ptr.PointTo(60),
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Enabled)
			assert.Equal(t, 4, result.MinimumApprovals)
			assert.Equal(t, 60, result.RetentionPeriodDays)
		})

		t.Run("Setting same Enabled value does not trigger role validation", func(t *testing.T) {
			configManager, _, tenant := SetupTenantConfigManager(t, nil)
			ctx := testutils.CreateCtxWithTenant(tenant)
			setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

			result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
				Enabled:          ptr.PointTo(true),
				MinimumApprovals: ptr.PointTo(3),
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Enabled)
			assert.Equal(t, 3, result.MinimumApprovals)
		})
	})
}
