package manager_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
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

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
	})

	dbRepository := sql.NewRepository(db)
	cfg := config.Config{Plugins: testutils.SetupMockPlugins(plugins...)}
	ctlg, err := catalog.New(t.Context(), cfg)
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
		cfg, err := configManager.GetDefaultKeystore(testutils.CreateCtxWithTenant(tenant))

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		var defaultKeystore model.DefaultKeystore

		err = json.Unmarshal(cfg.Value, &defaultKeystore)
		assert.NoError(t, err)

		assert.NotEmpty(t, defaultKeystore.LocalityID)
		assert.NotEmpty(t, defaultKeystore.CommonName)
		assert.NotEmpty(t, defaultKeystore.ManagementAccessData)
	})

	t.Run("Config Exists", func(t *testing.T) {
		// Arrange
		configManager, db, tenant := SetupTenantConfigManager(t, nil)

		tenantConfigRepo := sql.NewRepository(db)
		ksConfigJSON, err := json.Marshal(&model.DefaultKeystore{
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
		gotConfig, err := configManager.GetDefaultKeystore(testutils.CreateCtxWithTenant(tenant))

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, gotConfig)

		var defaultKeystore model.DefaultKeystore

		err = json.Unmarshal(gotConfig.Value, &defaultKeystore)
		assert.NoError(t, err)

		assert.Equal(t, testutils.TestLocalityID, defaultKeystore.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, defaultKeystore.CommonName)
		assert.Equal(t, testutils.TestRoleArn, defaultKeystore.ManagementAccessData["roleArn"])
		assert.Equal(t, testutils.TestTrustAnchorArn, defaultKeystore.ManagementAccessData["trustAnchorArn"])
		assert.Equal(t, testutils.TestProfileArn, defaultKeystore.ManagementAccessData["profileArn"])
	})
}

func TestSetDefaultKeystore(t *testing.T) {
	t.Run("DefaultKeystore tenant config not exists, set default keystore", func(t *testing.T) {
		// Arrange
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ksconfig := testutils.NewKeystoreConfig(func(_ *model.KeystoreConfiguration) {})
		ctx := testutils.CreateCtxWithTenant(tenant)

		// Act
		err := configManager.SetDefaultKeystore(ctx, ksconfig)

		// Assert
		assert.NoError(t, err)
		gotConfig, err := configManager.GetDefaultKeystore(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, gotConfig)

		var defaultKeystore model.DefaultKeystore

		err = json.Unmarshal(gotConfig.Value, &defaultKeystore)
		assert.NoError(t, err)
		assert.Equal(t, testutils.TestLocalityID, defaultKeystore.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, defaultKeystore.CommonName)
		assert.Equal(t, testutils.TestRoleArn, defaultKeystore.ManagementAccessData["roleArn"])
		assert.Equal(t, testutils.TestTrustAnchorArn, defaultKeystore.ManagementAccessData["trustAnchorArn"])
		assert.Equal(t, testutils.TestProfileArn, defaultKeystore.ManagementAccessData["profileArn"])
	})

	t.Run("Update existing default keystore config", func(t *testing.T) {
		// Arrange
		configManager, _, tenant := SetupTenantConfigManager(t, nil)
		ksconfig := testutils.NewKeystoreConfig(func(_ *model.KeystoreConfiguration) {})
		ctx := testutils.CreateCtxWithTenant(tenant)
		err := configManager.SetDefaultKeystore(ctx, ksconfig)
		assert.NoError(t, err)

		newLocalityID := uuid.NewString()
		newRoleArn := "arn:aws:iam::123456789012:role/ExampleRoleUpdated"
		newTrustAnchorID := "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/" + uuid.NewString()
		newProfileArn := "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/" + uuid.NewString()
		updatedConfigValue := map[string]any{
			"localityId": newLocalityID,
			"commonName": testutils.TestDefaultKeystoreCommonName,
			"managementAccessData": map[string]string{
				"roleArn":        newRoleArn,
				"trustAnchorArn": newTrustAnchorID,
				"profileArn":     newProfileArn,
			},
		}

		valueBytes, _ := json.Marshal(updatedConfigValue)
		updatedConfig := testutils.NewKeystoreConfig(func(ks *model.KeystoreConfiguration) {
			ks.Value = valueBytes
		})

		// Act
		err = configManager.SetDefaultKeystore(ctx, updatedConfig)

		// Assert
		assert.NoError(t, err)
		gotConfig, err := configManager.GetDefaultKeystore(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, gotConfig)

		var defaultKeystore model.DefaultKeystore

		err = json.Unmarshal(gotConfig.Value, &defaultKeystore)
		assert.NoError(t, err)
		assert.Equal(t, newLocalityID, defaultKeystore.LocalityID)
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, defaultKeystore.CommonName)
		assert.Equal(t, newRoleArn, defaultKeystore.ManagementAccessData["roleArn"])
		assert.Equal(t, newTrustAnchorID, defaultKeystore.ManagementAccessData["trustAnchorArn"])
		assert.Equal(t, newProfileArn, defaultKeystore.ManagementAccessData["profileArn"])
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

			ctlg, err := catalog.New(t.Context(), cfg)
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
