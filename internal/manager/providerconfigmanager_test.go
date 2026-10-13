package manager_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/multitenancy"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func SetupProviderManager(t *testing.T) (*manager.ProviderConfigManager, string, *multitenancy.DB) {
	t.Helper()

	svcRegistry := testutils.NewTestPlugins()
	cfg := &config.Config{}

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	m := manager.NewProviderConfigManager(
		svcRegistry,
		make(map[manager.ProviderCachedKey]*manager.ProviderConfig),
		manager.NewTenantConfigManager(r, svcRegistry, cfg, manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg), nil),
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

	roleManagementCfg, ok := ks["roleManagementConfig"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "test-uuid", roleManagementCfg["localityID"])
	assert.Equal(t, "default.kms.test", roleManagementCfg["commonName"])
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
		c.Purpose = model.CertificatePurposeHYOKManagement
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
				k.KeyType = cmkapi.KeyTypeHYOK
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
				k.KeyType = cmkapi.KeyTypeHYOK
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

func TestGetOrInitProvider_HYOKAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		disableHYOKCertAuth bool
		setupCert           bool
		expectAuthType      bool
		expectedAuthType    string
		expectCertFields    bool
		expectExpiration    bool
	}{
		{
			name:                "cert auth when DisableHYOKCertAuth is false",
			disableHYOKCertAuth: false,
			setupCert:           true,
			expectAuthType:      true,
			expectedAuthType:    constants.AuthTypeCertificate,
			expectCertFields:    true,
			expectExpiration:    true,
		},
		{
			name:                "no authType set when DisableHYOKCertAuth is true (plugin decides)",
			disableHYOKCertAuth: true,
			setupCert:           false,
			expectAuthType:      false,
			expectCertFields:    false,
			expectExpiration:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svcRegistry := testutils.NewTestPlugins()
			cfg := &config.Config{}
			cfg.Certificates.DisableHYOKCertAuth = tt.disableHYOKCertAuth

			db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
			r := sql.NewRepository(db)
			tenant := tenants[0]
			ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

			if tt.setupCert {
				cert := testutils.NewCertificate(func(c *model.Certificate) {
					c.Purpose = model.CertificatePurposeHYOKManagement
				})
				testutils.CreateTestEntities(ctx, t, r, cert)
			}

			m := manager.NewProviderConfigManager(
				svcRegistry,
				make(map[manager.ProviderCachedKey]*manager.ProviderConfig),
				manager.NewTenantConfigManager(r, svcRegistry, cfg, manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg), nil),
				manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg),
				manager.NewPool(r),
				r,
			)

			key := testutils.NewKey(func(k *model.Key) {
				k.KeyType = cmkapi.KeyTypeHYOK
				k.Provider = providerTest
			})

			provider, err := m.GetOrInitProvider(ctx, key)

			assert.NoError(t, err)
			assert.NotNil(t, provider)

			authType, hasAuthType := provider.Config.Values["authType"]
			assert.Equal(t, tt.expectAuthType, hasAuthType)
			if tt.expectAuthType {
				assert.Equal(t, tt.expectedAuthType, authType)
			}

			_, hasClientCert := provider.Config.Values["clientCert"]
			_, hasPrivateKey := provider.Config.Values["privateKey"]
			assert.Equal(t, tt.expectCertFields, hasClientCert)
			assert.Equal(t, tt.expectCertFields, hasPrivateKey)

			if tt.expectExpiration {
				// Cert auth: expiration comes from the certificate
				assert.True(t, provider.Expiration.After(time.Now()))
			} else {
				// No cert: expiration falls back to the default cache duration (~24h)
				assert.True(t, provider.Expiration.After(time.Now()))
				assert.True(t, provider.Expiration.Before(time.Now().Add(manager.DefaultProviderConfigCacheExpiration+time.Minute)))
			}
		})
	}
}

func TestGetOrInitProvider_ExpiredEntryIsReinitialized(t *testing.T) {
	svcRegistry := testutils.NewTestPlugins()
	cfg := &config.Config{}

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	cert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeHYOKManagement
	})
	testutils.CreateTestEntities(ctx, t, r, cert)

	expiredAt := time.Now().Add(-time.Second)
	expiredCfg := manager.NewProviderConfig(nil, nil, &expiredAt)

	compositeKey := manager.ProviderCachedKey{
		KeyStore: constants.HYOKKeyStore,
		Provider: providerTest,
		Tenant:   tenant,
	}

	m := manager.NewProviderConfigManager(
		svcRegistry,
		map[manager.ProviderCachedKey]*manager.ProviderConfig{
			compositeKey: expiredCfg,
		},
		manager.NewTenantConfigManager(r, svcRegistry, cfg, manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg), nil),
		manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg),
		manager.NewPool(r),
		r,
	)

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyType = cmkapi.KeyTypeHYOK
		k.Provider = providerTest
	})

	provider, err := m.GetOrInitProvider(ctx, key)

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.False(t, provider.IsExpired())
	assert.True(t, provider.Expiration.After(expiredAt))
}
