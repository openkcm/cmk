package manager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func SetupProviderManager(t *testing.T) (*manager.ProviderConfigManager, string, *multitenancy.DB) {
	t.Helper()

	ps, psCfg := testutils.NewTestPlugins(
		testplugins.NewKeystoreOperator(),
		testplugins.NewKeystoreManagement(),
	)

	cfg := &config.Config{
		Plugins: psCfg,
	}
	svcRegistry, err := cmkpluginregistry.New(t.Context(), cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(t, err)

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	m := manager.NewProviderConfigManager(
		svcRegistry,
		make(map[manager.ProviderCachedKey]*manager.ProviderConfig),
		manager.NewTenantConfigManager(r, svcRegistry, cfg),
		manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg),
		manager.NewPool(r),
		r,
	)
	return m, tenants[0], db
}

func TestGetPluginAlgorithm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "RSA3072 Algorithm",
			input:    "RSA3072",
			expected: "KEY_ALGORITHM_RSA3072",
		},
		{
			name:     "AES256 Algorithm",
			input:    "AES256",
			expected: "KEY_ALGORITHM_AES256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := manager.GetPluginAlgorithm(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateKeystore(t *testing.T) {
	m, _, _ := SetupProviderManager(t)
	provider, ks, err := m.CreateKeystore(t.Context())

	assert.NoError(t, err)
	assert.NotNil(t, ks)
	assert.Equal(t, providerTest, provider)
	assert.Equal(t, "test-uuid", ks["locality"])
	assert.Equal(t, "default.kms.test", ks["commonName"])
}

func TestFillKeystorePool(t *testing.T) {
	m, tenant, db := SetupProviderManager(t)
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	size := 2

	err := m.FillKeystorePool(t.Context(), size)
	assert.NoError(t, err)

	// Verify that keystore pool has been filled
	count, err := r.Count(ctx, &model.Keystore{}, *repo.NewQuery())
	assert.NoError(t, err)

	assert.Equal(t, size, count)
}

func TestGetOrInitProvider(t *testing.T) {
	m, tenant, db := SetupProviderManager(t)
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	cert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeTenantDefault
	})
	testutils.CreateTestEntities(ctx, t, r, cert)
	tests := []struct {
		name   string
		key    *model.Key
		assert func(t *testing.T, provider *manager.ProviderConfig, err error)
	}{
		{
			name: "Valid Provider",
			key: testutils.NewKey(func(k *model.Key) {
				k.KeyType = constants.KeyTypeHYOK
				k.Provider = providerTest
			}),
			assert: func(t *testing.T, provider *manager.ProviderConfig, err error) {
				t.Helper()

				assert.NoError(t, err)
				assert.NotNil(t, provider)
			},
		},
		{
			name: "Invalid Provider",
			key: testutils.NewKey(func(k *model.Key) {
				k.KeyType = constants.KeyTypeHYOK
				k.Provider = "GCP"
			}),
			assert: func(t *testing.T, provider *manager.ProviderConfig, err error) {
				t.Helper()

				assert.Error(t, err)
				assert.Nil(t, provider)
				assert.ErrorIs(t, err, manager.ErrPluginNotFound)
				assert.EqualError(t, err, "plugin not found: GCP")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
			provider, err := m.GetOrInitProvider(ctx, tt.key)
			tt.assert(t, provider, err)
		})
	}
}
