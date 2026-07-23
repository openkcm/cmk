package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	"github.com/openkcm/cmk/utils/ptr"
)

var ErrForced = errors.New("forced")

func SetupTenantConfigManager(t *testing.T, opts ...testplugins.RegistryOption) (*manager.TenantConfigManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)
	svcRegistry := testutils.NewTestPlugins(opts...)

	cfg := &config.Config{
		Certificates: config.Certificates{
			RootCertURL:  TestCertURL,
			ValidityDays: config.MinCertificateValidityDays,
		},
	}
	tenantManager := manager.NewTenantConfigManager(r, svcRegistry, cfg, nil)

	return tenantManager, db, tenants[0]
}

// SetupTenantConfigManagerWithRole creates a test tenant with a specific role
func SetupTenantConfigManagerWithRole(t *testing.T, role string, opts ...testplugins.RegistryOption) (*manager.TenantConfigManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithTenantRole(model.TenantRole(role)))

	r := sql.NewRepository(db)
	svcRegistry := testutils.NewTestPlugins(opts...)
	tenantManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil)

	return tenantManager, db, tenants[0]
}

func TestNewTenantConfigManager(t *testing.T) {
	m, _, _ := SetupTenantConfigManager(t)

	assert.NotNil(t, m)
}

// TestGetDefaultKeystore tests the GetDefaultKeystore method
func TestGetDefaultKeystore(t *testing.T) {
	t.Run("DefaultKeystore tenant config not exists, get from pool", func(t *testing.T) {
		// Arrange
		configManager, db, tenant := SetupTenantConfigManager(t)
		// Add a keystore configuration to the pool
		ctx := testutils.CreateCtxWithTenant(tenant)
		r := sql.NewRepository(db)

		// Create a fresh keystore config for this test to avoid pollution from other tests
		expectedLocalityID := uuid.NewString()
		localKsConfig := testutils.NewKeystore(func(k *model.Keystore) {
			keystoreConfig := testutils.NewKeystoreConfig(func(cfg *model.KeystoreConfig) {
				cfg.RoleManagementConfig.LocalityID = expectedLocalityID
			})
			configBytes, marshalErr := json.Marshal(keystoreConfig)
			assert.NoError(t, marshalErr)
			k.Config = configBytes
		})
		testutils.CreateTestEntities(ctx, t, r, localKsConfig)

		// Act
		keystore, err := configManager.GetDefaultKeystoreConfig(ctx)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, keystore)
		assert.Equal(t, expectedLocalityID, keystore.RoleManagementConfig.LocalityID)
		assert.NotEmpty(t, keystore.RoleManagementConfig.CommonName)
		assert.NotEmpty(t, keystore.RoleManagementConfig.AccessData)
	})

	t.Run("Config Exists", func(t *testing.T) {
		// Arrange
		configManager, db, tenant := SetupTenantConfigManager(t)

		tenantConfigRepo := sql.NewRepository(db)
		ksConfigJSON, err := json.Marshal(&model.KeystoreConfig{
			RoleManagementConfig: model.ManagementConfig{
				LocalityID: testutils.TestLocalityID,
				CommonName: testutils.TestDefaultKeystoreCommonName,
				AccessData: model.KeystoreAccessData{
					"roleArn":        testutils.TestRoleArn,
					"trustAnchorArn": testutils.TestTrustAnchorArn,
					"profileArn":     testutils.TestProfileArn,
				},
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
		assert.Equal(t, testutils.TestLocalityID, keystore.RoleManagementConfig.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, keystore.RoleManagementConfig.CommonName)
		assert.Equal(t, testutils.TestRoleArn, keystore.RoleManagementConfig.AccessData["roleArn"])
		assert.Equal(t, testutils.TestTrustAnchorArn, keystore.RoleManagementConfig.AccessData["trustAnchorArn"])
		assert.Equal(t, testutils.TestProfileArn, keystore.RoleManagementConfig.AccessData["profileArn"])
	})
}

func TestSetDefaultKeystore(t *testing.T) {
	t.Run("DefaultKeystore tenant config not exists, set default keystore", func(t *testing.T) {
		// Arrange
		configManager, _, tenant := SetupTenantConfigManager(t)
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

		assert.Equal(t, testutils.TestLocalityID, keystore.RoleManagementConfig.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, keystore.RoleManagementConfig.CommonName)
		assert.Equal(t, testutils.TestRoleArn, keystore.RoleManagementConfig.AccessData["roleArn"])
		assert.Equal(t, testutils.TestTrustAnchorArn, keystore.RoleManagementConfig.AccessData["trustAnchorArn"])
		assert.Equal(t, testutils.TestProfileArn, keystore.RoleManagementConfig.AccessData["profileArn"])
	})

	t.Run("Update existing default keystore config", func(t *testing.T) {
		// Arrange
		configManager, _, tenant := SetupTenantConfigManager(t)
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
			kc.RoleManagementConfig.LocalityID = newLocalityID
			kc.RoleManagementConfig.CommonName = testutils.TestDefaultKeystoreCommonName
			kc.RoleManagementConfig.AccessData = map[string]any{
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

		assert.Equal(t, newLocalityID, keystore.RoleManagementConfig.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, keystore.RoleManagementConfig.CommonName)
		assert.Equal(t, newRoleArn, keystore.RoleManagementConfig.AccessData["roleArn"])
		assert.Equal(t, newTrustAnchorID, keystore.RoleManagementConfig.AccessData["trustAnchorArn"])
		assert.Equal(t, newProfileArn, keystore.RoleManagementConfig.AccessData["profileArn"])
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
			svcRegistry := testutils.NewTestPlugins(
				testplugins.WithKeyManagement(
					testplugins.Name,
					testplugins.NewTestKeyManagement(tt.enabledPlugins, false),
				),
			)

			mgr := manager.NewTenantConfigManager(nil, svcRegistry, nil, nil)

			result := mgr.GetTenantConfigsHyokKeystore()
			assert.ElementsMatch(t, tt.expectedOutput, result.Provider)
			assert.IsNonDecreasing(t, result.Provider)
		})
	}
}

func TestGetTenantsKeystore(t *testing.T) {
	t.Run("Should get tenant keystores with hyok", func(t *testing.T) {
		m, _, tenant := SetupTenantConfigManager(t,
			testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, false)))
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.NotEmpty(t, res.HYOK)
	})

	t.Run("Should get tenant keystores with no hyok providers", func(t *testing.T) {
		m, _, tenant := SetupTenantConfigManager(t,
			testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(false, true)))
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Empty(t, res.HYOK)
		assert.False(t, res.AllowBYOK)
	})

	t.Run("Should keep BYOK disabled when feature gate is missing", func(t *testing.T) {
		m, _, tenant := SetupTenantConfigManager(t)
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.False(t, res.AllowBYOK)
	})

	t.Run("Should enable BYOK when allow-byok feature gate is true", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		r := sql.NewRepository(db)
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				FeatureGates: commoncfg.FeatureGates{
					"allow-byok": true,
				},
			},
		}
		m := manager.NewTenantConfigManager(r, nil, cfg, nil)
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.True(t, res.AllowBYOK)
	})

	t.Run("BYOK allowed, no stored keystore: regions from config", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		r := sql.NewRepository(db)

		regionsJSON, err := json.Marshal(testutils.SupportedRegions)
		assert.NoError(t, err)

		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				FeatureGates: commoncfg.FeatureGates{"allow-byok": true},
			},
			KeystorePool: config.KeystorePool{
				SupportedRegions: commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  string(regionsJSON),
				},
			},
		}
		m := manager.NewTenantConfigManager(r, nil, cfg, nil)
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Equal(t, testutils.SupportedRegions, res.BYOK.SupportedRegions)
	})

	t.Run("BYOK disabled, no stored keystore", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		r := sql.NewRepository(db)
		m := manager.NewTenantConfigManager(r, nil, &config.Config{}, nil)
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Nil(t, res.BYOK.SupportedRegions)
	})

	t.Run("BYOK allowed, no source ref", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				FeatureGates: commoncfg.FeatureGates{"allow-byok": true},
			},
		}
		m := manager.NewTenantConfigManager(sql.NewRepository(db), nil, cfg, nil)
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Nil(t, res.BYOK.SupportedRegions)
	})

	t.Run("stored keystore: regions from keystore", func(t *testing.T) {
		configManager, db, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		err := configManager.SetDefaultKeystore(ctx, testutils.NewKeystoreConfig(func(k *model.KeystoreConfig) {
			k.SupportedRegions = testutils.SupportedRegions
		}))
		assert.NoError(t, err)

		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				FeatureGates: commoncfg.FeatureGates{"allow-byok": true},
			},
		}
		m := manager.NewTenantConfigManager(sql.NewRepository(db), nil, cfg, nil)
		res, err := m.GetTenantsKeystores(ctx)
		assert.NoError(t, err)
		assert.Equal(t, testutils.SupportedRegions, res.BYOK.SupportedRegions)
	})

	t.Run("BYOK allowed, bad source ref", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				FeatureGates: commoncfg.FeatureGates{"allow-byok": true},
			},
			KeystorePool: config.KeystorePool{
				SupportedRegions: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					Value:  "/nonexistent/regions.json",
				},
			},
		}
		m := manager.NewTenantConfigManager(sql.NewRepository(db), nil, cfg, nil)
		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Nil(t, res.BYOK.SupportedRegions)
	})
}

func TestUpdateWorkflowConfig(t *testing.T) {
	// Helper to setup config for a tenant
	setupConfig := func(t *testing.T,
		mgr *manager.TenantConfigManager, ctx context.Context, cfg *model.WorkflowConfig,
	) {
		t.Helper()
		_, err := mgr.SetWorkflowConfig(ctx, cfg)
		assert.NoError(t, err)
	}

	t.Run("Should update workflow config with partial update", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
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
		configManager, _, tenant := SetupTenantConfigManager(t)
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
		configManager, _, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: ptr.PointTo(29),
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, manager.ErrRetentionLessThanMinimum)
	})

	t.Run("Should fail when defaultExpiryPeriodDays exceeds maxExpiryPeriodDays", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			DefaultExpiryPeriodDays: ptr.PointTo(15),
			MaxExpiryPeriodDays:     ptr.PointTo(10),
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, manager.ErrDefaultExpiryExceedsMax)
	})

	t.Run("Should succeed when defaultExpiryPeriodDays equals maxExpiryPeriodDays", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			DefaultExpiryPeriodDays: ptr.PointTo(10),
			MaxExpiryPeriodDays:     ptr.PointTo(10),
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 10, result.DefaultExpiryPeriodDays)
		assert.Equal(t, 10, result.MaxExpiryPeriodDays)
	})

	t.Run("Should fail when minimumApprovals is less than 2", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals: ptr.PointTo(1),
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, manager.ErrMinimumApprovalsTooLow)
	})

	t.Run("Should succeed when minimumApprovals equals 2", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)
		setupConfig(t, configManager, ctx, testutils.NewDefaultWorkflowConfig(true))

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals: ptr.PointTo(2),
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.MinimumApprovals)
	})

	t.Run("Should create default config when updating non-existent config", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		result, err := configManager.UpdateWorkflowConfig(ctx, &cmkapi.TenantWorkflowConfiguration{
			Enabled: ptr.PointTo(true),
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Enabled)
	})

	t.Run("Should handle nil update gracefully", func(t *testing.T) {
		configManager, _, tenant := SetupTenantConfigManager(t)
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
					configManager, _, tenant = SetupTenantConfigManagerWithRole(t, tt.role)
				} else {
					configManager, _, tenant = SetupTenantConfigManager(t)
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
			configManager, _, tenant := SetupTenantConfigManager(t)
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
				tenantpb.Role_ROLE_TEST.String())
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
			configManager, _, tenant := SetupTenantConfigManager(t)
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

// capturingKeystoreManagement wraps a TestKeystoreManagement and records every
// GrantTrust call so tests can assert on the Subject and Type values passed.
type capturingKeystoreManagement struct {
	inner       *testplugins.TestKeystoreManagement
	grantCalls  []*keystoremanagement.GrantTrustRequest
	removeCalls []*keystoremanagement.RemoveTrustRequest
}

var _ keystoremanagement.KeystoreManagement = (*capturingKeystoreManagement)(nil)

func newCapturingKeystoreManagement() *capturingKeystoreManagement {
	return &capturingKeystoreManagement{inner: testplugins.NewTestKeystoreManagement()}
}

func (c *capturingKeystoreManagement) ServiceInfo() api.Info {
	return c.inner.ServiceInfo()
}

func (c *capturingKeystoreManagement) CreateKeystore(
	ctx context.Context, req *keystoremanagement.CreateKeystoreRequest,
) (*keystoremanagement.CreateKeystoreResponse, error) {
	return c.inner.CreateKeystore(ctx, req)
}

func (c *capturingKeystoreManagement) DeleteKeystore(
	ctx context.Context, req *keystoremanagement.DeleteKeystoreRequest,
) (*keystoremanagement.DeleteKeystoreResponse, error) {
	return c.inner.DeleteKeystore(ctx, req)
}

func (c *capturingKeystoreManagement) GrantTrust(
	ctx context.Context, req *keystoremanagement.GrantTrustRequest,
) (*keystoremanagement.GrantTrustResponse, error) {
	c.grantCalls = append(c.grantCalls, req)
	return c.inner.GrantTrust(ctx, req)
}

func (c *capturingKeystoreManagement) RemoveTrust(
	ctx context.Context, req *keystoremanagement.RemoveTrustRequest,
) (*keystoremanagement.RemoveTrustResponse, error) {
	c.removeCalls = append(c.removeCalls, req)
	return c.inner.RemoveTrust(ctx, req)
}

// grantCallsOfType returns GrantTrust calls matching the given TrustType.
func (c *capturingKeystoreManagement) grantCallsOfType(
	trustType keystoremanagement.TrustType,
) []*keystoremanagement.GrantTrustRequest {
	var out []*keystoremanagement.GrantTrustRequest
	for _, call := range c.grantCalls {
		if call.Type == trustType {
			out = append(out, call)
		}
	}
	return out
}

// setupTenantConfigManagerWithCerts creates a TenantConfigManager backed by a
// CertificateManager so that ensureKeystoreProvisioned is exercised.
// It also pre-persists a role-management certificate so that
// getDefaultKeystoreClientCert finds it without triggering IssueCertificate
// (the test issuer returns an empty chain).
// cfg is the config to use; if nil, a default is created.
func setupTenantConfigManagerWithCerts(
	t *testing.T,
	cfg *config.Config,
	opts ...testplugins.RegistryOption,
) (*manager.TenantConfigManager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	svcRegistry := testutils.NewTestPlugins(opts...)

	if cfg == nil {
		cfg = &config.Config{
			Certificates: config.Certificates{
				RootCertURL:  testutils.TestCertURL,
				ValidityDays: config.MinCertificateValidityDays,
			},
		}
	}

	// Ensure getCryptoCertificates returns an empty list (so syncCryptoAccessData
	// exits early) without failing on "no credential found".
	if cfg.CryptoLayer.CertX509Trusts.Source == "" {
		cfg.CryptoLayer.CertX509Trusts = commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "[]",
		}
	}

	certManager := manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, cfg, certManager)

	// Pre-persist a role-management cert so getDefaultKeystoreClientCert doesn't
	// attempt to call IssueCertificate (the test stub returns an empty chain).
	ctx := testutils.CreateCtxWithTenant(tenants[0])
	roleManagementCert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeRoleManagement
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
	testutils.CreateTestEntities(ctx, t, r, roleManagementCert)

	return tenantConfigManager, db, tenants[0]
}

// storeKsConfig marshals ks and persists it as the tenant's default keystore config.
func storeKsConfig(t *testing.T, db *multitenancy.DB, tenant string, ks *model.KeystoreConfig) {
	t.Helper()
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)
	b, err := json.Marshal(ks)
	require.NoError(t, err)
	require.NoError(t, r.Set(ctx, &model.TenantConfig{Key: constants.DefaultKeyStore, Value: b}))
}

func TestEnsureKeystoreProvisioned(t *testing.T) {
	const testPrefix = "test_prefix_"

	cfg := &config.Config{
		Certificates: config.Certificates{
			RootCertURL:             testutils.TestCertURL,
			ValidityDays:            config.MinCertificateValidityDays,
			DefaultTenantCertPrefix: testPrefix,
		},
	}

	t.Run("skips GrantTrust(MANAGEMENT) when KeyManagementConfig already set", func(t *testing.T) {
		// Arrange: create a keystore config that already has KeyManagementConfig.LocalityID set.
		capture := newCapturingKeystoreManagement()
		m, db, tenant := setupTenantConfigManagerWithCerts(
			t, cfg,
			testplugins.WithKeystoreManagement(testplugins.Name, capture),
		)
		ctx := testutils.CreateCtxWithTenant(tenant)

		// Use a full ksConfig (both RoleManagementConfig and KeyManagementConfig set).
		storeKsConfig(t, db, tenant, testutils.NewKeystoreConfig(func(_ *model.KeystoreConfig) {}))

		// Act
		_, err := m.GetDefaultKeystoreConfig(ctx)
		require.NoError(t, err)

		// Assert: no MANAGEMENT GrantTrust call should have been made.
		mgmtCalls := capture.grantCallsOfType(keystoremanagement.TrustTypeManagement)
		assert.Empty(t, mgmtCalls, "expected no GrantTrust(MANAGEMENT) call when KeyManagementConfig already provisioned")
	})

	t.Run("calls GrantTrust(MANAGEMENT) with Subject=prefix+tenantID when KeyManagementConfig missing", func(t *testing.T) {
		// Arrange: store a ksConfig with empty KeyManagementConfig (only RoleManagementConfig set).
		capture := newCapturingKeystoreManagement()
		m, db, tenant := setupTenantConfigManagerWithCerts(
			t, cfg,
			testplugins.WithKeystoreManagement(testplugins.Name, capture),
		)
		ctx := testutils.CreateCtxWithTenant(tenant)

		ksWithoutKeyMgmt := testutils.NewKeystoreConfig(func(kc *model.KeystoreConfig) {
			kc.KeyManagementConfig = model.ManagementConfig{} // clear it
		})
		storeKsConfig(t, db, tenant, ksWithoutKeyMgmt)

		// Act
		_, err := m.GetDefaultKeystoreConfig(ctx)
		require.NoError(t, err)

		// Assert: exactly one MANAGEMENT call with Subject == prefix+tenantID.
		mgmtCalls := capture.grantCallsOfType(keystoremanagement.TrustTypeManagement)
		require.Len(t, mgmtCalls, 1)
		assert.Equal(t, keystoremanagement.TrustTypeManagement, mgmtCalls[0].Type)
		assert.Equal(t, testPrefix+manager.DefaultKeystoreCertInfix+tenant, mgmtCalls[0].Subject)
	})

	t.Run("stores KeyManagementConfig with correct CommonName after GrantTrust(MANAGEMENT)", func(t *testing.T) {
		// Arrange
		capture := newCapturingKeystoreManagement()
		m, db, tenant := setupTenantConfigManagerWithCerts(
			t, cfg,
			testplugins.WithKeystoreManagement(testplugins.Name, capture),
		)
		ctx := testutils.CreateCtxWithTenant(tenant)

		ksWithoutKeyMgmt := testutils.NewKeystoreConfig(func(kc *model.KeystoreConfig) {
			kc.KeyManagementConfig = model.ManagementConfig{}
		})
		storeKsConfig(t, db, tenant, ksWithoutKeyMgmt)

		// Act
		result, err := m.GetDefaultKeystoreConfig(ctx)
		require.NoError(t, err)

		// Assert: the returned config has the expected CommonName.
		expectedCN := testPrefix + manager.DefaultKeystoreCertInfix + tenant
		assert.Equal(t, expectedCN, result.KeyManagementConfig.CommonName)
		assert.NotEmpty(t, result.KeyManagementConfig.LocalityID)
	})
}

// cryptoCertCfg returns a *config.Config whose CryptoLayer contains one CryptoCert
// with the given name and CN prefix, embedded as inline YAML.
func cryptoCertCfg(name, cnPrefix string) *config.Config {
	yamlVal := "- name: " + name + "\n  subject:\n    commonNamePrefix: " + cnPrefix + "\n"
	return &config.Config{
		Certificates: config.Certificates{
			RootCertURL:             testutils.TestCertURL,
			ValidityDays:            config.MinCertificateValidityDays,
			DefaultTenantCertPrefix: "test_prefix_",
		},
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  yamlVal,
			},
		},
	}
}

func TestSyncCryptoAccessData(t *testing.T) {
	const (
		certName = "crypto-cert-1"
		cnPrefix = "crypto_"
	)

	t.Run("skips GrantTrust(CRYPTO) when subject already matches", func(t *testing.T) {
		capture := newCapturingKeystoreManagement()
		cfg := cryptoCertCfg(certName, cnPrefix)
		m, db, tenant := setupTenantConfigManagerWithCerts(
			t, cfg,
			testplugins.WithKeystoreManagement(testplugins.Name, capture),
		)
		ctx := testutils.CreateCtxWithTenant(tenant)

		// Pre-populate CryptoAccessData with the correct subject so no sync needed.
		// The subject is formatted as pkix.Name.String(), e.g. "CN=crypto_<tenant>".
		expectedSubject := "CN=" + cnPrefix + tenant
		ksConfig := testutils.NewKeystoreConfig(func(kc *model.KeystoreConfig) {
			kc.CryptoAccessData = map[string]model.CryptoConfig{
				certName: {Subject: expectedSubject, AccessData: model.KeystoreAccessData{}},
			}
		})
		storeKsConfig(t, db, tenant, ksConfig)

		_, err := m.GetDefaultKeystoreConfig(ctx)
		require.NoError(t, err)

		cryptoCalls := capture.grantCallsOfType(keystoremanagement.TrustTypeCrypto)
		assert.Empty(t, cryptoCalls, "expected no GrantTrust(CRYPTO) call when subject already matches")
		assert.Empty(t, capture.removeCalls, "expected no RemoveTrust call when subject already matches")
	})

	t.Run("calls GrantTrust(CRYPTO) when entry missing", func(t *testing.T) {
		capture := newCapturingKeystoreManagement()
		cfg := cryptoCertCfg(certName, cnPrefix)
		m, db, tenant := setupTenantConfigManagerWithCerts(
			t, cfg,
			testplugins.WithKeystoreManagement(testplugins.Name, capture),
		)
		ctx := testutils.CreateCtxWithTenant(tenant)

		// Store ksConfig with no CryptoAccessData.
		ksConfig := testutils.NewKeystoreConfig(func(kc *model.KeystoreConfig) {
			kc.CryptoAccessData = nil
		})
		storeKsConfig(t, db, tenant, ksConfig)

		_, err := m.GetDefaultKeystoreConfig(ctx)
		require.NoError(t, err)

		cryptoCalls := capture.grantCallsOfType(keystoremanagement.TrustTypeCrypto)
		require.Len(t, cryptoCalls, 1, "expected exactly one GrantTrust(CRYPTO) call")
		assert.Equal(t, keystoremanagement.TrustTypeCrypto, cryptoCalls[0].Type)
		assert.Equal(t, "CN="+cnPrefix+tenant, cryptoCalls[0].Subject)
		assert.Empty(t, capture.removeCalls, "expected no RemoveTrust call when entry is new")
	})

	t.Run("skips GrantTrust(CRYPTO) when entry already exists regardless of subject", func(t *testing.T) {
		capture := newCapturingKeystoreManagement()
		cfg := cryptoCertCfg(certName, cnPrefix)
		m, db, tenant := setupTenantConfigManagerWithCerts(
			t, cfg,
			testplugins.WithKeystoreManagement(testplugins.Name, capture),
		)
		ctx := testutils.CreateCtxWithTenant(tenant)

		// Store ksConfig with an existing entry (even with a different subject).
		ksConfig := testutils.NewKeystoreConfig(func(kc *model.KeystoreConfig) {
			kc.CryptoAccessData = map[string]model.CryptoConfig{
				certName: {Subject: "old_subject", AccessData: model.KeystoreAccessData{}},
			}
		})
		storeKsConfig(t, db, tenant, ksConfig)

		_, err := m.GetDefaultKeystoreConfig(ctx)
		require.NoError(t, err)

		assert.Empty(t, capture.removeCalls, "expected no RemoveTrust call")
		assert.Empty(t, capture.grantCallsOfType(keystoremanagement.TrustTypeCrypto), "expected no GrantTrust(CRYPTO) call when entry already exists")
	})
}
