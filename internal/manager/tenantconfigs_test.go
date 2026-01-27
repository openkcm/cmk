package manager_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
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

	tenantManager := manager.NewTenantConfigManager(dbRepository, ctlg)

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

			mgr := manager.NewTenantConfigManager(nil, ctlg)

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
