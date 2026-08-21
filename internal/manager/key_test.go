package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"gopkg.in/yaml.v3"

	grpcstatus "google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

const (
	testRegionUSEast1 = "us-east-1"
)

func SetupKeyTest(t *testing.T, opts ...testplugins.RegistryOption) (
	*manager.KeyManager,
	repo.Repo,
	context.Context,
	*model.KeyConfiguration,
	*model.Keystore,
) {
	t.Helper()

	db, tenants, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	svcRegistry := testutils.NewTestPlugins(opts...)
	cryptoCerts := []config.CryptoCert{
		{
			Name: "crypto-1",
			Subject: config.CryptoCertSubject{
				Locality:           []string{"Berlin"},
				OrganizationalUnit: []string{"OU1", "OU2"},
				Organization:       []string{"TestOrg"},
				Country:            []string{"DE"},
				CommonNamePrefix:   "test_",
			},
			RootCA: "https://example.com/root.crt",
		},
	}
	cryptoCertsBytes, err := yaml.Marshal(cryptoCerts)
	require.NoError(t, err)

	cfg := &config.Config{
		Database: dbConf,
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(cryptoCertsBytes),
			},
		},
	}

	cmkAuditor := auditor.New(ctx, cfg)

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	assert.NoError(t, err)

	certManager := manager.NewCertificateManager(ctx, r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, certManager)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)

	km := manager.NewKeyManager(
		r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, eventFactory, cmkAuditor, nil,
	)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})

	ks := testutils.NewKeystore(func(_ *model.Keystore) {})
	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		tenantDefaultCert,
		testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeRoleManagement
			c.CommonName = testutils.TestDefaultKeystoreCommonName
		}),
		testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeKeyManagement
			c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
		}),
		ks,
	)

	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

	return km, r, ctx, keyConfig, ks
}

func createTestSystemManagedKey(t *testing.T, km *manager.KeyManager, r repo.Repo, ctx context.Context, keyConfigID uuid.UUID) *model.Key {
	t.Helper()
	// Seed a fully-provisioned DEFAULT_KEYSTORE config so NeedsDefaultKeystoreProvisioning
	// returns false and the key takes the normal creation path instead of PENDING_CREATION.
	seedDefaultKeystore(t, r, ctx)

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigID
	})

	createdKey, err := km.Create(ctx, key)
	require.NoError(t, err)

	return createdKey
}

// seedDefaultKeystore persists a fully-provisioned DEFAULT_KEYSTORE config as flat rows so that
// NeedsDefaultKeystoreProvisioning returns false and BYOK keys take the normal creation
// path instead of PENDING_CREATION.
func seedDefaultKeystore(t *testing.T, r repo.Repo, ctx context.Context) {
	t.Helper()
	//nolint:contextcheck
	tcm := manager.NewTenantConfigManager(r, testutils.NewTestPlugins(), &config.Config{}, nil)
	err := tcm.SetDefaultKeystore(ctx, testutils.NewKeystoreConfig(func(_ *model.KeystoreConfig) {}))
	require.NoError(t, err)
}

func createTestHYOKKey(
	t *testing.T,
	km *manager.KeyManager,
	ctx context.Context,
	keyConfigID uuid.UUID,
	provider *testplugins.TestKeyManagement,
) *model.Key {
	t.Helper()
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	cryptoAccessData := model.KeyAccessData{
		"crypto-1": {
			CertificateSubject: new("CN=test_tenant0,OU=OU1/OU2,O=TestOrg,L=Berlin,C=DE"),
			AdditionalProperties: map[string]any{
				"someKey": "someValue",
			},
		},
	}
	cryptoBytes, err := json.Marshal(cryptoAccessData)
	require.NoError(t, err)

	keyProvider, err := provider.CreateKey(ctx, &keymanagement.CreateKeyRequest{
		KeyType: keymanagement.HYOK,
	})
	require.NoError(t, err)

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigID
		k.KeyType = cmkapi.KeyTypeHYOK
		k.NativeID = &keyProvider.KeyID
		k.ManagementAccessData = hyokInfo
		k.Provider = providerTest
		k.CryptoAccessData = cryptoBytes
	})

	createdKey, err := km.Create(ctx, key)
	require.NoError(t, err)

	return createdKey
}

func createTestBYOKKey(t *testing.T, r repo.Repo, ctx context.Context, keyConfigID uuid.UUID, state cmkapi.KeyState, provider *testplugins.TestKeyManagement) *model.Key {
	t.Helper()

	keyProvider, err := provider.CreateKey(ctx, &keymanagement.CreateKeyRequest{
		KeyType: keymanagement.BYOK,
	})

	require.NoError(t, err)
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigID
		k.KeyType = cmkapi.KeyTypeBYOK
		k.State = state
		k.NativeID = &keyProvider.KeyID
	})

	testutils.CreateTestEntities(ctx, t, r, key)

	return key
}

func TestCreate(t *testing.T) {
	keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
	km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))

	keyProvider, err := keyProviderPlugin.CreateKey(t.Context(), &keymanagement.CreateKeyRequest{
		KeyType: keymanagement.HYOK,
	})
	require.NoError(t, err)

	// Seed a provisioned DEFAULT_KEYSTORE config so NeedsDefaultKeystoreProvisioning returns false
	// and BYOK keys take the normal creation path through the provider.
	seedDefaultKeystore(t, r, ctx)

	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	tests := []struct {
		name         string
		key          func() *model.Key
		wantErr      bool
		errMsg       string
		wantState    cmkapi.KeyState
		wantNativeID bool // true = NativeID must be non-nil
	}{
		{
			name: "Valid managed key creation",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
				})
			},
			wantErr:      false,
			wantNativeID: true,
		},
		{
			name: "Invalid provider",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = cmkapi.KeyTypeHYOK
					k.NativeID = &keyProvider.KeyID
					k.ManagementAccessData = hyokInfo
					k.Provider = "INVALID"
				})
			},
			wantErr: true,
		},
		{
			name: "Valid HYOK key creation",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = cmkapi.KeyTypeHYOK
					k.NativeID = &keyProvider.KeyID
					k.ManagementAccessData = hyokInfo
					k.Provider = providerTest
				})
			},
			wantErr:      false,
			wantNativeID: true,
		},
		{
			name: "HYOK key creation wrong access data",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = cmkapi.KeyTypeHYOK
					k.NativeID = &keyProvider.KeyID
					k.ManagementAccessData = []byte("{\"invalid\": \"data\"}")
					k.Provider = providerTest
				})
			},
			wantErr: true,
			errMsg:  "failed to authenticate with the keystore provider",
		},
		{
			name: "HYOK key creation key not found",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = cmkapi.KeyTypeHYOK
					k.NativeID = new("invalid-key-id")
					k.ManagementAccessData = hyokInfo
					k.Provider = providerTest
				})
			},
			wantErr: true,
			errMsg:  "HYOK provider key not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.key()
			result, err := km.Create(ctx, key)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, key.ID, result.ID)
				if tt.wantNativeID {
					assert.NotNil(t, result.NativeID)
				}
				if tt.wantState != "" {
					assert.Equal(t, tt.wantState, result.State)
				}
			}
		})
	}

	t.Run("Should have unique name on a keyconfig", func(t *testing.T) {
		name := uuid.NewString()
		key1 := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.Name = name
		})

		key2 := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.Name = name
		})

		_, err := km.Create(ctx, key1)
		assert.NoError(t, err)

		_, err = km.Create(ctx, key2)
		assert.ErrorIs(t, err, repo.ErrUniqueConstraint)
	})

	t.Run("Should allow same name on different keyconfig", func(t *testing.T) {
		name := uuid.NewString()
		keyConfig1 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		key1 := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig1.ID
			k.Name = name
		})

		keyConfig2 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		key2 := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig2.ID
			k.Name = name
		})

		testutils.CreateTestEntities(ctx, t, r, keyConfig1, keyConfig2)
		localCtx := testutils.InjectBusinessUserDataIntoContext(
			ctx,
			uuid.NewString(),
			[]string{keyConfig1.AdminGroup.IAMIdentifier, keyConfig2.AdminGroup.IAMIdentifier, keyConfig.AdminGroup.IAMIdentifier},
		)

		_, err := km.Create(localCtx, key1)
		assert.NoError(t, err)

		_, err = km.Create(localCtx, key2)
		assert.NoError(t, err)
	})
}

func TestHYOKRegistrationCertificateSubject(t *testing.T) {
	keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
	km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))

	keyProvider, err := keyProviderPlugin.CreateKey(t.Context(), &keymanagement.CreateKeyRequest{
		KeyType: keymanagement.HYOK,
	})
	require.NoError(t, err)

	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	t.Run("should add certificate subject to crypto access data when cert name matches", func(t *testing.T) {
		cryptoAccessData := model.KeyAccessData{
			"crypto-1": {
				AdditionalProperties: map[string]any{
					"someKey": "someValue",
				},
			},
		}
		cryptoBytes, err := json.Marshal(cryptoAccessData)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.NativeID = &keyProvider.KeyID
			k.ManagementAccessData = hyokInfo
			k.Provider = providerTest
			k.CryptoAccessData = cryptoBytes
		})

		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)

		resultData := createdKey.GetCryptoAccessData()
		require.NotNil(t, resultData)
		require.Contains(t, resultData, "crypto-1")
		assert.NotNil(t, resultData["crypto-1"].CertificateSubject)

		subject := *resultData["crypto-1"].CertificateSubject
		assert.Contains(t, subject, "OU=OU1/OU2")
		assert.Contains(t, subject, "O=TestOrg")
		assert.Contains(t, subject, "L=Berlin")
		assert.Contains(t, subject, "C=DE")
	})

	t.Run("should not add certificate subject when cert name does not match", func(t *testing.T) {
		cryptoAccessData := model.KeyAccessData{
			"non-existent-cert": {
				AdditionalProperties: map[string]any{
					"someKey": "someValue",
				},
			},
		}
		cryptoBytes, err := json.Marshal(cryptoAccessData)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.NativeID = &keyProvider.KeyID
			k.ManagementAccessData = hyokInfo
			k.Provider = providerTest
			k.CryptoAccessData = cryptoBytes
		})

		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)

		resultData := createdKey.GetCryptoAccessData()
		require.NotNil(t, resultData)
		require.Contains(t, resultData, "non-existent-cert")
		assert.Empty(t, resultData["crypto-1"].CertificateSubject)
	})

	t.Run("should handle HYOK key with no crypto access data", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.NativeID = &keyProvider.KeyID
			k.ManagementAccessData = hyokInfo
			k.Provider = providerTest
		})

		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)
		assert.NotNil(t, createdKey)
	})
}

func TestSetFirstKeyPrimary(t *testing.T) {
	km, r, ctx, keyConfig, _ := SetupKeyTest(t)

	t.Run("Should set first key as primary", func(t *testing.T) {
		createdKey1 := createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)

		_ = createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)

		resKeyConfig := &model.KeyConfiguration{ID: keyConfig.ID, AdminGroup: model.Group{ID: uuid.New()}}
		_, err := r.First(ctx, resKeyConfig, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, createdKey1.ID, *resKeyConfig.PrimaryKeyID)
	})
}

func TestEditableCryptoData(t *testing.T) {
	km, r, ctx, _, _ := SetupKeyTest(t)

	regionEditable := "region1"
	regionNonEditable := "region2"

	cryptoData, err := json.Marshal(model.KeyAccessData{
		regionEditable:    cmkapi.KeyAccessDetailsRegion{},
		regionNonEditable: cmkapi.KeyAccessDetailsRegion{},
	})
	require.NoError(t, err)

	t.Run("Should all be editable on non primary Key", func(t *testing.T) {
		kc := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

		sysFailed := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = new(kc.ID)
			sys.Region = regionEditable
			sys.Status = cmkapi.SystemStatusFAILED
		})

		sysConnected := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = new(kc.ID)
			sys.Region = regionNonEditable
			sys.Status = cmkapi.SystemStatusCONNECTED
		})

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyType = cmkapi.KeyTypeHYOK
			k.CryptoAccessData = cryptoData
			k.KeyConfigurationID = kc.ID
		})

		testutils.CreateTestEntities(ctx, t, r, kc, sysFailed, sysConnected, key)
		ctx := testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{kc.AdminGroup.IAMIdentifier})

		key, err = km.Get(ctx, key.ID)
		assert.NoError(t, err)

		assert.True(t, key.EditableRegions[regionEditable])
		assert.True(t, key.EditableRegions[regionNonEditable])
	})

	t.Run("Should be editable on pkey only on failed regions", func(t *testing.T) {
		keyID := uuid.New()
		kc := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = &keyID
		})

		sysFailed := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = new(kc.ID)
			sys.Region = regionEditable
			sys.Status = cmkapi.SystemStatusFAILED
		})

		sysConnected := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = new(kc.ID)
			sys.Region = regionNonEditable
			sys.Status = cmkapi.SystemStatusCONNECTED
		})

		testutils.CreateTestEntities(ctx, t, r, kc, sysFailed, sysConnected)

		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.CryptoAccessData = cryptoData
			k.KeyConfigurationID = kc.ID
		})
		localCtx := testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{kc.AdminGroup.IAMIdentifier})

		testutils.CreateTestEntities(ctx, t, r, key)

		key, err = km.Get(localCtx, key.ID)
		assert.NoError(t, err)

		assert.True(t, key.EditableRegions[regionEditable])
		assert.False(t, key.EditableRegions[regionNonEditable])
	})
}

func TestGet(t *testing.T) {
	keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
	km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))

	createdKey := createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)
	hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)
	byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

	tests := []struct {
		name    string
		keyID   uuid.UUID
		wantErr bool
	}{
		{
			name:    "Existing managed key",
			keyID:   createdKey.ID,
			wantErr: false,
		},
		{
			name:    "Existing hyok key",
			keyID:   hyokKey.ID,
			wantErr: false,
		},
		{
			name:    "ExistingBYOKKey",
			keyID:   byokKey.ID,
			wantErr: false,
		},
		{
			name:    "Non-existent key",
			keyID:   uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := km.Get(ctx, tt.keyID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.keyID, result.ID)
			}
		})
	}
}

func TestHYOKSync(t *testing.T) {
	t.Run("HYOK key state is enabled after creation", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		gotKey, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, gotKey.State)
	})

	t.Run("HYOK key state syncs after provider disable", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		disableKey(t, km, ctx, hyokKey)

		key, err = km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateDISABLED, key.State)
		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
	})

	t.Run("hyok state syncs after provider disable", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		provider, err := km.GetOrInitProvider(ctx, hyokKey)
		assert.NoError(t, err)
		_, err = provider.Client.DisableKey(ctx, &keymanagement.DisableKeyRequest{
			Parameters: keymanagement.RequestParameters{
				KeyID:  *hyokKey.NativeID,
				Config: common.KeystoreConfig{Values: provider.Config.Values},
			},
		})
		assert.NoError(t, err)
		err = km.SyncHYOKKeys(ctx)
		assert.NoError(t, err)
		key, err = km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateDISABLED, key.State)
		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
	})

	t.Run("hyok sync delete", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		deleteKey(t, km, ctx, hyokKey)
		err = km.SyncHYOKKeys(ctx)
		assert.NoError(t, err)
		key, err = km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStatePENDINGDELETION, key.State)
	})

	t.Run("hyok sync delete/enable", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		deleteKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStatePENDINGDELETION)

		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateENABLED)
	})

	t.Run("hyok sync delete/disable", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		deleteKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStatePENDINGDELETION)

		disableKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateDISABLED)
	})

	t.Run("hyok state syncs on key deleted", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		key.NativeID = new("invalid-key-id")
		_, err = r.Patch(ctx, key, *repo.NewQuery())
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateDELETED)
	})

	t.Run("hyok state syncs on auth change", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		key.ManagementAccessData = []byte("{\"invalid\": \"data\"}")
		_, err = r.Patch(ctx, key, *repo.NewQuery())
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateFORBIDDEN)
	})

	t.Run("hyok state disable twice then enable twice", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.KeyStateENABLED, key.State)

		disableKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateDISABLED)

		disableKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateDISABLED)

		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateENABLED)

		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, cmkapi.KeyStateENABLED)
	})
}

func syncAndVerifyState(t *testing.T, km *manager.KeyManager, ctx context.Context, hyokKey *model.Key, expectedState cmkapi.KeyState) {
	t.Helper()
	err := km.SyncHYOKKeys(ctx)
	assert.NoError(t, err)
	key, err := km.Get(ctx, hyokKey.ID)
	assert.NoError(t, err)
	assert.Equal(t, expectedState, key.State)
}

func disableKey(t *testing.T, km *manager.KeyManager, ctx context.Context, hyokKey *model.Key) {
	t.Helper()
	provider, err := km.GetOrInitProvider(ctx, hyokKey)
	assert.NoError(t, err)
	_, err = provider.Client.DisableKey(ctx, &keymanagement.DisableKeyRequest{
		Parameters: keymanagement.RequestParameters{
			KeyID:  *hyokKey.NativeID,
			Config: common.KeystoreConfig{Values: provider.Config.Values},
		},
	})
	assert.NoError(t, err)
}

func deleteKey(t *testing.T, km *manager.KeyManager, ctx context.Context, hyokKey *model.Key) {
	t.Helper()
	provider, err := km.GetOrInitProvider(ctx, hyokKey)
	assert.NoError(t, err)
	_, err = provider.Client.DeleteKey(ctx, &keymanagement.DeleteKeyRequest{
		Parameters: keymanagement.RequestParameters{
			KeyID:  *hyokKey.NativeID,
			Config: common.KeystoreConfig{Values: provider.Config.Values},
		},
	})
	assert.NoError(t, err)
}

func enableKey(t *testing.T, km *manager.KeyManager, ctx context.Context, hyokKey *model.Key) error {
	t.Helper()
	provider, err := km.GetOrInitProvider(ctx, hyokKey)
	assert.NoError(t, err)
	_, err = provider.Client.EnableKey(ctx, &keymanagement.EnableKeyRequest{
		Parameters: keymanagement.RequestParameters{
			KeyID:  *hyokKey.NativeID,
			Config: common.KeystoreConfig{Values: provider.Config.Values},
		},
	})
	assert.NoError(t, err)

	return err
}

func TestList(t *testing.T) {
	km, r, ctx, keyConfig, _ := SetupKeyTest(t)

	createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)
	createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)

	sys := testutils.NewSystem(func(sys *model.System) {
		sys.Status = cmkapi.SystemStatusFAILED
		sys.KeyConfigurationID = new(keyConfig.ID)
	})

	testutils.CreateTestEntities(ctx, t, r, sys)

	tests := []struct {
		name          string
		skip          int
		top           int
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "List all keys",
			skip:          0,
			top:           10,
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "List with pagination",
			skip:          0,
			top:           1,
			expectedCount: 2,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := km.GetKeys(ctx, keyConfig.ID, repo.Pagination{Skip: tt.skip, Top: tt.top, Count: true})

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, results)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, results)
				assert.Equal(t, tt.expectedCount, total)
				assert.LessOrEqual(t, len(results), tt.top)
			}
		})
	}
}

//nolint:nestif
func TestUpdate(t *testing.T) {
	keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
	km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
	createdKey := createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)

	tests := []struct {
		name     string
		keyID    uuid.UUID
		keyPatch cmkapi.KeyPatch
		wantErr  bool
	}{
		{
			name:  "Update name and description",
			keyID: createdKey.ID,
			keyPatch: cmkapi.KeyPatch{
				Name:        new("updated-name"),
				Description: new("Updated description"),
			},
			wantErr: false,
		},
		{
			name:  "Disable key",
			keyID: createdKey.ID,
			keyPatch: cmkapi.KeyPatch{
				Enabled: new(false),
			},
			wantErr: false,
		},
		{
			name:  "Enable key",
			keyID: createdKey.ID,
			keyPatch: cmkapi.KeyPatch{
				Enabled: new(true),
			},
			wantErr: false,
		},
		{
			name:  "Non-existent key",
			keyID: uuid.New(),
			keyPatch: cmkapi.KeyPatch{
				Name: new("new-name"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := km.UpdateKey(ctx, tt.keyID, tt.keyPatch)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				updatedKey, err := km.Get(ctx, tt.keyID)
				assert.NoError(t, err)

				if tt.keyPatch.Name != nil {
					assert.Equal(t, *tt.keyPatch.Name, updatedKey.Name)
				}

				if tt.keyPatch.Description != nil {
					assert.Equal(t, *tt.keyPatch.Description, updatedKey.Description)
				}

				if tt.keyPatch.Enabled != nil {
					assert.Equal(t, *tt.keyPatch.Enabled, updatedKey.State == cmkapi.KeyStateENABLED)

					if *tt.keyPatch.Enabled {
						assert.Equal(t, cmkapi.KeyStateENABLED, updatedKey.State)
					} else {
						assert.Equal(t, cmkapi.KeyStateDISABLED, updatedKey.State)
					}
				}
			}
		})
	}

	t.Run("Should allow adding more crypto regions with uneditable ones", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig)
		ctx := testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		key := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)
		keyConfig.PrimaryKeyID = &key.ID

		_, err := r.Patch(ctx, keyConfig, *repo.NewQuery())
		assert.NoError(t, err)

		// Create system to make region not editable
		keyPatch := cmkapi.KeyPatch{
			AccessDetails: &cmkapi.KeyAccessDetails{
				Crypto: &map[string]cmkapi.KeyAccessDetailsRegion{
					"crypto-2": {
						AdditionalProperties: map[string]any{"key": "value"},
					},
				},
			},
		}
		system1 := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusCONNECTED
			s.Region = "crypto-1"
		})
		testutils.CreateTestEntities(ctx, t, r, system1)
		res, err := km.UpdateKey(ctx, key.ID, keyPatch)
		assert.NoError(t, err)

		cryptoData := res.GetCryptoAccessData()
		assert.Contains(t, cryptoData, "crypto-2")
	})

	t.Run("Should only override sent fields on registered regions", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig)
		ctx := testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		key := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)
		keyConfig.PrimaryKeyID = &key.ID

		_, err := r.Patch(ctx, keyConfig, *repo.NewQuery())
		assert.NoError(t, err)

		keyPatch := cmkapi.KeyPatch{
			AccessDetails: &cmkapi.KeyAccessDetails{
				Crypto: &map[string]cmkapi.KeyAccessDetailsRegion{
					"crypto-1": {
						AdditionalProperties: map[string]any{
							"someKey": "patchValue",
						},
					},
				},
			},
		}
		res, err := km.UpdateKey(ctx, key.ID, keyPatch)
		assert.NoError(t, err)

		cryptoData := res.GetCryptoAccessData()
		assert.NotNil(t, cryptoData["crypto-1"].CertificateSubject)
		someKeyVal, ok := cryptoData["crypto-1"].Get("someKey")
		assert.True(t, ok)
		assert.Equal(t, "patchValue", someKeyVal)
	})
}

func TestDelete(t *testing.T) {
	keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
	km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))

	createdKey := createTestSystemManagedKey(t, km, r, ctx, keyConfig.ID)
	createdPrimaryKey, err := km.Create(ctx, testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	}))
	require.NoError(t, err)
	byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

	keyID := uuid.New()
	keyConfigWSystems := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = new(keyID)
	})
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = new(keyConfigWSystems.ID)
	})
	keyFailSystems := testutils.NewKey(func(k *model.Key) {
		k.ID = keyID
		k.KeyConfigurationID = keyConfigWSystems.ID
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfigWSystems, sys, keyFailSystems)

	tests := []struct {
		name    string
		keyID   uuid.UUID
		wantErr bool
	}{
		{
			name:    "Delete existing key",
			keyID:   createdKey.ID,
			wantErr: false,
		},
		{
			name:    "Should fail on delete pkey with connected systems",
			keyID:   keyFailSystems.ID,
			wantErr: true,
		},
		{
			name:    "Delete primary key",
			keyID:   createdPrimaryKey.ID,
			wantErr: false,
		},
		{
			name:    "DeleteExistingBYOKKey",
			keyID:   byokKey.ID,
			wantErr: false,
		},
		{
			name:    "Delete non-existent key",
			keyID:   uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := km.Delete(ctx, tt.keyID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				_, err := km.Get(ctx, tt.keyID)
				assert.Error(t, err)
			}
		})
	}
}

func TestGetImportParams(t *testing.T) {
	cachedPublicKeyFromDB := "mock-public-key-from-database"
	fetchedPublicKeyFromProvider := "mock-public-key-from-provider"

	t.Run("Success_NilImportParams", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, fetchedPublicKeyFromProvider, got.PublicKeyPEM)
	})

	t.Run("Success_ImportParamsNotExpired", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = new(time.Now().Add(24 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)
		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cachedPublicKeyFromDB, got.PublicKeyPEM)
	})

	t.Run("Success_NilExpires", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = nil
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)
		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cachedPublicKeyFromDB, got.PublicKeyPEM)
	})

	t.Run("Success_ImportParamsExpired", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = new(time.Now().Add(-1 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)
		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, fetchedPublicKeyFromProvider, got.PublicKeyPEM)
	})

	t.Run("Error_InvalidKeyType", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		sysKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)
		_, err := km.GetImportParams(ctx, sysKey.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyTypeForImportParams)
		assert.Contains(t, err.Error(), "key type")
	})

	t.Run("Error_InvalidKeyState", func(t *testing.T) {
		km, r, ctx, keyConfig, _ := SetupKeyTest(t)

		byokEnabledKey := testutils.NewKey(func(k *model.Key) {
			k.KeyType = cmkapi.KeyTypeBYOK
			k.State = cmkapi.KeyStateENABLED
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctx, t, r, byokEnabledKey)

		_, err := km.GetImportParams(ctx, byokEnabledKey.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyStateForImportParams)
		assert.Contains(t, err.Error(), "key state")
	})

	t.Run("Error_KeyNotFound", func(t *testing.T) {
		km, _, ctx, _, _ := SetupKeyTest(t)
		_, err := km.GetImportParams(ctx, uuid.New())
		assert.Error(t, err)
	})

	t.Run("Error_Unauthorized_WrongGroup", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		ctxWrongGroup := testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{"different_group"})
		_, err := km.GetImportParams(ctxWrongGroup, byokKey.ID)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})
}

func TestImportKeyMaterial(t *testing.T) {
	validMaterial := "dGVzdC1rZXktbWF0ZXJpYWw="

	t.Run("ImportParamsMissing", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))

		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		_, err := km.ImportKeyMaterial(ctx, byokKey.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrMissingOrExpiredImportParams)
		assert.Contains(t, err.Error(), "import parameters missing or expired")
	})

	t.Run("ImportParamsExpired", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		paramsJSON, err := json.Marshal(map[string]any{
			"providerParams": "test-provider-params",
		})
		assert.NoError(t, err)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.ProviderParameters = paramsJSON
			ip.Expires = new(time.Now().Add(-1 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)

		_, err = km.ImportKeyMaterial(ctx, byokKey.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrMissingOrExpiredImportParams)
		assert.Contains(t, err.Error(), "import parameters missing or expired")
	})

	t.Run("Success", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		paramsJSON, err := json.Marshal(map[string]any{
			"providerParams": "test-provider-params",
		})
		assert.NoError(t, err)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.ProviderParameters = paramsJSON
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)

		_, err = km.ImportKeyMaterial(ctx, byokKey.ID, validMaterial)

		assert.NoError(t, err)
	})

	t.Run("EmptyWrappedKeyMaterial", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		_, err := km.ImportKeyMaterial(ctx, byokKey.ID, "")

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrEmptyKeyMaterial)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("InvalidBase64WrappedKeyMaterial", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)

		_, err := km.ImportKeyMaterial(ctx, byokKey.ID, "not-base64")

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidBase64KeyMaterial)
		assert.Contains(t, err.Error(), "base64")
	})

	t.Run("InvalidKeyType", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, _, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		sysKey := createTestHYOKKey(t, km, ctx, keyConfig.ID, keyProviderPlugin)

		_, err := km.ImportKeyMaterial(ctx, sysKey.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyTypeForImportKeyMaterial)
		assert.Contains(t, err.Error(), "key type")
	})

	t.Run("InvalidKeyState", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		enabledBYOK := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStateENABLED, keyProviderPlugin)

		paramsJSON, err := json.Marshal(map[string]any{
			"providerParams": "test-provider-params",
		})
		assert.NoError(t, err)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = enabledBYOK.ID
			ip.ProviderParameters = paramsJSON
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)

		_, err = km.ImportKeyMaterial(ctx, enabledBYOK.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyStateForImportKeyMaterial)
		assert.Contains(t, err.Error(), "key state")
	})

	t.Run("KeyNotFound", func(t *testing.T) {
		km, _, ctx, _, _ := SetupKeyTest(t)

		_, err := km.ImportKeyMaterial(ctx, uuid.New(), validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrGetKeyDB)
	})

	t.Run("Error_Unauthorized_WrongGroup", func(t *testing.T) {
		keyProviderPlugin := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugin))
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGIMPORT, keyProviderPlugin)
		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.Expires = new(time.Now().Add(24 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)

		ctxWrongGroup := testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{"different_group"})
		_, err := km.ImportKeyMaterial(ctxWrongGroup, byokKey.ID, validMaterial)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})
}

func TestKeyRotationTime(t *testing.T) {
	// Setup plugin with custom rotation time
	keyProviderPlugins := testplugins.NewTestKeyManagement(true, true)
	keyProvider, err := keyProviderPlugins.CreateKey(t.Context(), &keymanagement.CreateKeyRequest{
		KeyType: keymanagement.HYOK,
	})
	assert.NoError(t, err)

	// Use custom setup similar to SetupKeyTest but with our plugin instance
	db, tenants, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	svcRegistry := testutils.NewTestPlugins(testplugins.WithKeyManagement(testplugins.Name, keyProviderPlugins))
	cryptoCerts := []config.CryptoCert{
		{
			Name: "crypto-1",
			Subject: config.CryptoCertSubject{
				CommonNamePrefix: "test_",
			},
			RootCA: "https://example.com/root.crt",
		},
	}
	cryptoCertsBytes, err := yaml.Marshal(cryptoCerts)
	require.NoError(t, err)

	cfg := &config.Config{
		Database: dbConf,
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(cryptoCertsBytes),
			},
		},
	}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)
	certManager := manager.NewCertificateManager(ctx, r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, certManager)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)
	km := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor, nil)

	// Create test data
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})
	keystoreDefaultCert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeRoleManagement
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
	ksConfig := testutils.NewKeystore(func(_ *model.Keystore) {})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, tenantDefaultCert, keystoreDefaultCert, ksConfig)

	// Inject client data for auth
	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

	t.Run("Register HYOK key - should use rotation time from keystore", func(t *testing.T) {
		// Create HYOK key
		hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.Name = "test-hyok-key"
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.Algorithm = cmkapi.KeyAlgorithmAES256
			k.Provider = testplugins.Name
			k.Region = testRegionUSEast1
			k.NativeID = &keyProvider.KeyID
			k.ManagementAccessData = hyokInfo
		})

		// Register key (which should create initial version with keystore rotation time)
		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)
		require.NotNil(t, createdKey)

		// Fetch key versions - query directly from repo
		versions, count, err := repo.ListAndCount(
			ctx, r, repo.Pagination{Skip: 0, Top: 10, Count: true},
			model.KeyVersion{},
			repo.NewQuery().Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("key_id", createdKey.ID),
			)),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		require.Len(t, versions, 1)

		// Verify rotation time matches keystore time (not current time)
		version := versions[0]
		assert.False(t, version.RotatedAt.IsZero(), "RotatedAt should be set")
		assert.NotEqual(t, version.RotatedAt, time.Now().UTC(),
			"RotatedAt should match keystore rotation time, not current time")
	})

	t.Run("Detect key rotation - should use rotation time from keystore", func(t *testing.T) {
		// Create initial key with version
		rotationTime := time.Date(2025, 6, 20, 14, 45, 0, 0, time.UTC)
		hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
		require.NoError(t, err)

		keyProvider, err := keyProviderPlugins.CreateKey(ctx, &keymanagement.CreateKeyRequest{
			KeyType: keymanagement.HYOK,
		})
		assert.NoError(t, err)
		// Register the key in the plugin first
		require.NoError(t, keyProviderPlugins.RotateKey(keyProvider.KeyID, "version-1", &rotationTime))

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyType = cmkapi.KeyTypeHYOK
			k.Provider = testplugins.Name
			k.NativeID = &keyProvider.KeyID
			k.KeyConfigurationID = keyConfig.ID
			k.ManagementAccessData = hyokInfo
			k.KeyVersions = []model.KeyVersion{}
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		// Manually trigger sync for our key (use SyncHYOKKeys which syncs all)
		err = km.SyncHYOKKeys(ctx)
		require.NoError(t, err)

		// Fetch versions again
		versions, count, err := repo.ListAndCount(
			ctx, r, repo.Pagination{Skip: 0, Top: 10, Count: true},
			model.KeyVersion{},
			repo.NewQuery().Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("key_id", key.ID),
			)),
		)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "Should have 2 versions after rotation")
		require.Len(t, versions, 2)

		// Find the new version
		var newVersion *model.KeyVersion
		for _, v := range versions {
			if v.NativeID == "version-1" {
				newVersion = v
				break
			}
		}
		require.NotNil(t, newVersion, "Should find the new version")

		// Verify rotation time matches keystore time for new version
		assert.False(t, newVersion.RotatedAt.IsZero(), "RotatedAt should be set")
		assert.Equal(t, rotationTime.Unix(), newVersion.RotatedAt.Unix(),
			"New version RotatedAt should match keystore rotation time")

		// Verify it's the rotation time from keystore, not current time
		now := time.Now().UTC()
		timeDiff := now.Sub(newVersion.RotatedAt)
		assert.Greater(t, timeDiff.Hours(), float64(24*30*6), // More than 6 months ago
			"RotatedAt should be the keystore time (2025-06-20), not current time")
	})

	t.Run("Fallback to current time when keystore doesn't provide rotation time", func(t *testing.T) {
		// Register the key in the plugin first
		keyProvider, err := keyProviderPlugins.CreateKey(ctx, &keymanagement.CreateKeyRequest{
			KeyType: keymanagement.HYOK,
		})
		assert.NoError(t, err)

		// Rotate key but don't set rotation time (empty string)
		require.NoError(t, keyProviderPlugins.RotateKey(keyProvider.KeyID, "version-1", nil))

		// Create HYOK key
		hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.Name = "test-hyok-no-time"
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.Algorithm = cmkapi.KeyAlgorithmAES256
			k.Provider = testplugins.Name
			k.Region = testRegionUSEast1
			k.NativeID = &keyProvider.KeyID
			k.ManagementAccessData = hyokInfo
		})

		beforeCreate := time.Now().UTC()
		createdKey, err := km.Create(ctx, key)
		afterCreate := time.Now().UTC()

		require.NoError(t, err)
		require.NotNil(t, createdKey)

		// Fetch version
		versions, _, err := repo.ListAndCount(
			ctx, r, repo.Pagination{Skip: 0, Top: 10, Count: true},
			model.KeyVersion{},
			repo.NewQuery().Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("key_id", createdKey.ID),
			)),
		)
		require.NoError(t, err)
		require.Len(t, versions, 2)

		// Verify rotation time is current time (between before and after)
		version := versions[0]
		assert.False(t, version.RotatedAt.IsZero(), "RotatedAt should be set")
		assert.True(t, version.RotatedAt.After(beforeCreate) || version.RotatedAt.Equal(beforeCreate),
			"RotatedAt should be current time when keystore doesn't provide it")
		assert.True(t, version.RotatedAt.Before(afterCreate) || version.RotatedAt.Equal(afterCreate),
			"RotatedAt should be current time when keystore doesn't provide it")
	})
}

func TestHandleSystemsOnKeyRotation(t *testing.T) {
	km, r, ctx, keyConfig, _ := SetupKeyTest(t)

	// Setup test: Create a primary key with some systems
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	primaryKey := testutils.NewKey(func(k *model.Key) {
		k.Name = "primary-key"
		k.KeyConfigurationID = keyConfig.ID
		k.KeyType = cmkapi.KeyTypeHYOK
		k.Algorithm = cmkapi.KeyAlgorithmAES256
		k.Provider = testplugins.Name
		k.Region = testRegionUSEast1
		k.NativeID = new("primary-key-native-id")
		k.ManagementAccessData = hyokInfo
	})

	keyConfig.PrimaryKeyID = new(primaryKey.ID)
	_, err = r.Patch(ctx, keyConfig, *repo.NewQuery())
	assert.NoError(t, err)

	nonPrimaryKey := testutils.NewKey(func(k *model.Key) {
		k.Name = "non-primary-key"
		k.KeyConfigurationID = keyConfig.ID
		k.KeyType = cmkapi.KeyTypeHYOK
		k.Algorithm = cmkapi.KeyAlgorithmAES256
		k.Provider = testplugins.Name
		k.Region = testRegionUSEast1
		k.NativeID = new("non-primary-key-native-id")
		k.ManagementAccessData = hyokInfo
	})

	// Create systems linked to this key configuration
	system1 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = &keyConfig.ID
		s.Status = cmkapi.SystemStatusCONNECTED
	})
	system2 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = &keyConfig.ID
		s.Status = cmkapi.SystemStatusCONNECTED
	})

	testutils.CreateTestEntities(ctx, t, r, primaryKey, nonPrimaryKey, system1, system2)

	t.Run("primary key rotation triggers SYSTEM_KEY_ROTATE events", func(t *testing.T) {
		// Simulate rotation detection by calling handleNewKeyVersion
		// First, setup plugin to return a new version
		rotationTime := time.Now().UTC()

		// Count events before
		eventsBefore, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// Trigger rotation by creating a new version via the internal method
		// We'll use the exported method from export_test.go
		err = km.ExportedHandleNewKeyVersion(ctx, primaryKey, &keymanagement.GetKeyVersionsResponse{
			Versions: []keymanagement.KeyVersion{
				{
					ID:           "new-version-id",
					CreationTime: &rotationTime,
				},
			},
		})
		require.NoError(t, err)

		// Count events after
		eventsAfter, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// Should have created 2 new rotation events (one per system)
		assert.Equal(t, eventsBefore+2, eventsAfter,
			"Should create SYSTEM_KEY_ROTATE events for both connected systems")
	})

	t.Run("non-primary key rotation does NOT trigger events", func(t *testing.T) {
		rotationTime := time.Now().UTC()

		// Count events before
		eventsBefore, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// Trigger rotation for non-primary key
		err = km.ExportedHandleNewKeyVersion(ctx, primaryKey, &keymanagement.GetKeyVersionsResponse{
			Versions: []keymanagement.KeyVersion{
				{
					ID:           "non-primary-new-version",
					CreationTime: &rotationTime,
				},
			},
		})
		require.NoError(t, err)

		// Count events after
		eventsAfter, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// Should NOT have created any new rotation events
		assert.Equal(t, eventsBefore, eventsAfter,
			"Should NOT create SYSTEM_KEY_ROTATE events for non-primary keys")
	})

	t.Run("handles keys with no connected systems", func(t *testing.T) {
		// Create a new key config with no systems
		emptyKeyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, emptyKeyConfig)

		emptyKey := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = emptyKeyConfig.ID
			k.KeyType = cmkapi.KeyTypeHYOK
			k.Provider = testplugins.Name
			k.Region = testRegionUSEast1
			k.NativeID = new("empty-key-native-id")
			k.ManagementAccessData = hyokInfo
			k.IsPrimary = true
		})
		testutils.CreateTestEntities(ctx, t, r, emptyKey)

		rotationTime := time.Now().UTC()

		eventsBefore, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// Should not error even with no systems
		err = km.ExportedHandleNewKeyVersion(ctx, primaryKey, &keymanagement.GetKeyVersionsResponse{
			Versions: []keymanagement.KeyVersion{
				{
					ID:           "empty-key-new-version",
					CreationTime: &rotationTime,
				},
			},
		})
		require.NoError(t, err)

		eventsAfter, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// No new events created (no systems to notify)
		assert.Equal(t, eventsBefore, eventsAfter)
	})
}

// Helper function to count events of a specific type
func countEvents(ctx context.Context, r repo.Repo, eventType string) (int, error) {
	_, count, err := repo.ListAndCount(
		ctx, r,
		repo.Pagination{Skip: 0, Top: 1000, Count: true},
		model.Event{},
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where("type", eventType),
		)),
	)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// failingNTimesKeyManagement wraps TestKeyManagement and returns failErr for the
// first failCount calls to CreateKey, then delegates to the inner implementation.
type failingNTimesKeyManagement struct {
	inner     *testplugins.TestKeyManagement
	failCount int
	callCount int
	failErr   error
}

var _ keymanagement.KeyManagement = (*failingNTimesKeyManagement)(nil)

var (
	errNonAuthTest          = errors.New("some other error")
	errMockAsyncUnavailable = errors.New("redis unavailable")
)

func (f *failingNTimesKeyManagement) ServiceInfo() api.Info {
	return f.inner.ServiceInfo()
}

func (f *failingNTimesKeyManagement) GetKey(ctx context.Context, req *keymanagement.GetKeyRequest) (*keymanagement.GetKeyResponse, error) {
	return f.inner.GetKey(ctx, req)
}

func (f *failingNTimesKeyManagement) GetKeyVersions(ctx context.Context, req *keymanagement.GetKeyVersionsRequest) (*keymanagement.GetKeyVersionsResponse, error) {
	return f.inner.GetKeyVersions(ctx, req)
}

func (f *failingNTimesKeyManagement) CreateKey(ctx context.Context, req *keymanagement.CreateKeyRequest) (*keymanagement.CreateKeyResponse, error) {
	if f.callCount < f.failCount {
		f.callCount++
		return nil, f.failErr
	}
	return f.inner.CreateKey(ctx, req)
}

func (f *failingNTimesKeyManagement) DeleteKey(ctx context.Context, req *keymanagement.DeleteKeyRequest) (*keymanagement.DeleteKeyResponse, error) {
	return f.inner.DeleteKey(ctx, req)
}

func (f *failingNTimesKeyManagement) EnableKey(ctx context.Context, req *keymanagement.EnableKeyRequest) (*keymanagement.EnableKeyResponse, error) {
	return f.inner.EnableKey(ctx, req)
}

func (f *failingNTimesKeyManagement) DisableKey(ctx context.Context, req *keymanagement.DisableKeyRequest) (*keymanagement.DisableKeyResponse, error) {
	return f.inner.DisableKey(ctx, req)
}

func (f *failingNTimesKeyManagement) GetImportParameters(ctx context.Context, req *keymanagement.GetImportParametersRequest) (*keymanagement.GetImportParametersResponse, error) {
	return f.inner.GetImportParameters(ctx, req)
}

func (f *failingNTimesKeyManagement) ImportKeyMaterial(ctx context.Context, req *keymanagement.ImportKeyMaterialRequest) (*keymanagement.ImportKeyMaterialResponse, error) {
	return f.inner.ImportKeyMaterial(ctx, req)
}

func (f *failingNTimesKeyManagement) ValidateKey(ctx context.Context, req *keymanagement.ValidateKeyRequest) (*keymanagement.ValidateKeyResponse, error) {
	return f.inner.ValidateKey(ctx, req)
}

func (f *failingNTimesKeyManagement) ValidateKeyAccessData(ctx context.Context, req *keymanagement.ValidateKeyAccessDataRequest) (*keymanagement.ValidateKeyAccessDataResponse, error) {
	return f.inner.ValidateKeyAccessData(ctx, req)
}

func (f *failingNTimesKeyManagement) TransformCryptoAccessData(ctx context.Context, req *keymanagement.TransformCryptoAccessDataRequest) (*keymanagement.TransformCryptoAccessDataResponse, error) {
	return f.inner.TransformCryptoAccessData(ctx, req)
}

func (f *failingNTimesKeyManagement) ExtractKeyRegion(ctx context.Context, req *keymanagement.ExtractKeyRegionRequest) (*keymanagement.ExtractKeyRegionResponse, error) {
	return f.inner.ExtractKeyRegion(ctx, req)
}

func TestCreateManagedProviderKeyRetry(t *testing.T) {
	// Zero out the retry delays so the test runs in milliseconds, not minutes.
	original := *manager.CreateKeyRetryDelay
	*manager.CreateKeyRetryDelay = 0
	t.Cleanup(func() { *manager.CreateKeyRetryDelay = original })
	originalMax := *manager.CreateKeyMaxDelay
	*manager.CreateKeyMaxDelay = 0
	t.Cleanup(func() { *manager.CreateKeyMaxDelay = originalMax })

	t.Run("succeeds on second attempt after one auth failure", func(t *testing.T) {
		plugin := &failingNTimesKeyManagement{
			inner:     testplugins.NewTestKeyManagement(true, true),
			failCount: 1,
			failErr:   keymanagement.ErrProviderAuthenticationFailed,
		}
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, plugin))

		// Seed a provisioned DEFAULT_KEYSTORE config so NeedsDefaultKeystoreProvisioning returns false.
		seedDefaultKeystore(t, r, ctx)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})

		result, err := km.Create(ctx, key)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.NativeID)
		assert.Equal(t, 1, plugin.callCount, "expected exactly one failure before success")
	})

	t.Run("fails after exhausting all retry attempts", func(t *testing.T) {
		// failCount=100 always fails — exceeds createKeyRetryAttempts (5).
		plugin := &failingNTimesKeyManagement{
			inner:     testplugins.NewTestKeyManagement(true, true),
			failCount: 100,
			failErr:   keymanagement.ErrProviderAuthenticationFailed,
		}
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, plugin))

		// Seed a provisioned DEFAULT_KEYSTORE config so NeedsDefaultKeystoreProvisioning returns false.
		seedDefaultKeystore(t, r, ctx)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})

		result, err := km.Create(ctx, key)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, keymanagement.ErrProviderAuthenticationFailed)
	})

	t.Run("does not retry on non-auth error", func(t *testing.T) {
		plugin := &failingNTimesKeyManagement{
			inner:     testplugins.NewTestKeyManagement(true, true),
			failCount: 100, // would succeed on attempt 101 if retried
			failErr:   errNonAuthTest,
		}
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, plugin))

		// Seed a provisioned DEFAULT_KEYSTORE config so NeedsDefaultKeystoreProvisioning returns false.
		seedDefaultKeystore(t, r, ctx)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})

		result, err := km.Create(ctx, key)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, 1, plugin.callCount, "expected only one attempt for non-auth error")
	})
}

func TestCreateBYOKPendingCreation(t *testing.T) {
	// SetupKeyTest does not pre-populate the tenant's default keystore config
	// (only the pool entry), so NeedsDefaultKeystoreProvisioning returns true.
	km, r, ctx, keyConfig, _ := SetupKeyTest(t)

	t.Run("BYOK key is created in PENDING_CREATION when provisioning not done", func(t *testing.T) {
		// Arrange
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
		})

		// Act
		result, err := km.Create(ctx, key)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, cmkapi.KeyStatePENDINGCREATION, result.State)
		assert.Nil(t, result.NativeID, "NativeID must be nil until provider creation completes")

		// Verify persisted state
		dbKey := &model.Key{ID: result.ID}
		found, err := r.First(ctx, dbKey, *repo.NewQuery())
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, cmkapi.KeyStatePENDINGCREATION, dbKey.State)
	})

	t.Run("PENDING_CREATION BYOK key is not set as primary", func(t *testing.T) {
		// Arrange: create a fresh key config so this is the first key for it
		freshKeyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, freshKeyConfig)
		localCtx := testutils.InjectBusinessUserDataIntoContext(
			ctx, uuid.NewString(),
			[]string{freshKeyConfig.AdminGroup.IAMIdentifier, keyConfig.AdminGroup.IAMIdentifier},
		)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = freshKeyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
		})

		// Act
		result, err := km.Create(localCtx, key)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, cmkapi.KeyStatePENDINGCREATION, result.State)

		// The key config should have no primary key since PENDING_CREATION keys skip that step
		kc := &model.KeyConfiguration{ID: freshKeyConfig.ID}
		found, err := r.First(localCtx, kc, *repo.NewQuery())
		require.NoError(t, err)
		require.True(t, found)
		assert.Nil(t, kc.PrimaryKeyID, "PENDING_CREATION key must not become primary")
	})
}

func TestUpdateKeyPendingCreationGuard(t *testing.T) {
	provider := testplugins.NewTestKeyManagement(true, true)
	km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, provider))

	t.Run("enable/disable rejected when key is in PENDING_CREATION state", func(t *testing.T) {
		// Arrange: store a BYOK key directly in PENDING_CREATION state
		key := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGCREATION, provider)

		// Act: try to enable
		_, err := km.UpdateKey(ctx, key.ID, cmkapi.KeyPatch{Enabled: new(true)})

		// Assert
		assert.ErrorIs(t, err, manager.ErrKeyInPendingState)
	})

	t.Run("enable/disable rejected when disabling PENDING_CREATION key", func(t *testing.T) {
		// Arrange
		key := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGCREATION, provider)

		// Act: try to disable
		_, err := km.UpdateKey(ctx, key.ID, cmkapi.KeyPatch{Enabled: new(false)})

		// Assert
		assert.ErrorIs(t, err, manager.ErrKeyInPendingState)
	})

	t.Run("name update allowed on PENDING_CREATION key", func(t *testing.T) {
		// Arrange
		key := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGCREATION, provider)
		newName := uuid.NewString()

		// Act: name update does not involve Enabled field — should succeed
		result, err := km.UpdateKey(ctx, key.ID, cmkapi.KeyPatch{Name: new(newName)})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, newName, result.Name)
	})
}

func TestSyncPendingCreationKey(t *testing.T) {
	// Override the timeout to 0 so keys are immediately considered timed out
	// when that specific sub-test needs it.

	t.Run("transitions key to PENDING_IMPORT when provisioning succeeds", func(t *testing.T) {
		km, r, ctx, keyConfig, _ := SetupKeyTest(t)

		// Arrange: persist a PENDING_CREATION key directly (bypassing Create)
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
			k.State = cmkapi.KeyStatePENDINGCREATION
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		// Arrange: store a provisioned keystore config so GetOrInitProvider succeeds
		seedDefaultKeystore(t, r, ctx)

		// Act
		syncErr := km.SyncPendingCreationKey(ctx, key.ID)

		// Assert
		require.NoError(t, syncErr)
		dbKey := &model.Key{ID: key.ID}
		found, err := r.First(ctx, dbKey, *repo.NewQuery())
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, cmkapi.KeyStatePENDINGIMPORT, dbKey.State,
			"key should transition to PENDING_IMPORT after successful provisioning")
		assert.NotNil(t, dbKey.NativeID, "NativeID should be set after provider key creation")
	})

	t.Run("transitions key to ERROR on timeout", func(t *testing.T) {
		// Override timeout to near-zero for this test
		original := *manager.PendingCreationTimeout
		*manager.PendingCreationTimeout = time.Nanosecond
		t.Cleanup(func() { *manager.PendingCreationTimeout = original })

		km, r, ctx, keyConfig, _ := SetupKeyTest(t)

		// Arrange: persist a PENDING_CREATION key
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
			k.State = cmkapi.KeyStatePENDINGCREATION
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		// Act: a tiny sleep to ensure the key's CreatedAt is older than 1ns
		time.Sleep(time.Millisecond)
		syncErr := km.SyncPendingCreationKey(ctx, key.ID)

		// Assert
		require.NoError(t, syncErr)
		dbKey := &model.Key{ID: key.ID}
		found, err := r.First(ctx, dbKey, *repo.NewQuery())
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, cmkapi.KeyStateERROR, dbKey.State,
			"key should transition to ERROR after timeout")
	})

	t.Run("transitions key to ERROR when keystore pool is drained", func(t *testing.T) {
		km, r, ctx, keyConfig, ks := SetupKeyTest(t)

		// Drain the keystore pool by deleting the pool entry seeded by SetupKeyTest.
		// With no pool entries and no stored DEFAULT_KEYSTORE config, GetOrInitProvider returns
		// ErrPoolIsDrained — which is non-recoverable, so the key should transition to ERROR.
		ck := repo.NewCompositeKey().Where(repo.IDField, ks.ID)
		_, err := r.Delete(ctx, &model.Keystore{}, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
			k.State = cmkapi.KeyStatePENDINGCREATION
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		// Act
		syncErr := km.SyncPendingCreationKey(ctx, key.ID)

		// Assert: no error returned (non-retryable errors are handled internally)
		require.NoError(t, syncErr)
		dbKey := &model.Key{ID: key.ID}
		found, err := r.First(ctx, dbKey, *repo.NewQuery())
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, cmkapi.KeyStateERROR, dbKey.State,
			"key should transition to ERROR when the keystore pool is drained,_,_,_")
		assert.NotNil(t, dbKey.ErrorDetail, "error detail should be populated")
	})

	t.Run("recovers key when CreateKey returns AlreadyExists", func(t *testing.T) {
		// Arrange: a plugin whose CreateKey returns AlreadyExists (prior partial run),
		// but whose GetKey returns the pre-existing key's NativeID.
		provider := testplugins.NewTestKeyManagement(false, true)
		keyProvider, err := provider.CreateKey(t.Context(), &keymanagement.CreateKeyRequest{
			KeyType: keymanagement.BYOK,
		})
		assert.NoError(t, err)

		alreadyExistsPlugin := &alreadyExistsKeyManagement{
			TestKeyManagement: provider,
			nativeID:          keyProvider.KeyID,
		}
		km, r, ctx, keyConfig, _ := SetupKeyTest(t,
			testplugins.WithKeyManagement(testplugins.Name, alreadyExistsPlugin),
		)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
			k.State = cmkapi.KeyStatePENDINGCREATION
			k.NativeID = &keyProvider.KeyID
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		seedDefaultKeystore(t, r, ctx)

		// Act
		syncErr := km.SyncPendingCreationKey(ctx, key.ID)

		// Assert
		require.NoError(t, syncErr)
		dbKey := &model.Key{ID: key.ID}
		found, err := r.First(ctx, dbKey, *repo.NewQuery())
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, cmkapi.KeyStatePENDINGIMPORT, dbKey.State,
			"key should transition to PENDING_IMPORT after AlreadyExists recovery")
		require.NotNil(t, dbKey.NativeID,
			"NativeID should be set after recovering from AlreadyExists")
		assert.Equal(t, keyProvider.KeyID, *dbKey.NativeID,
			"NativeID should match the pre-existing key in the provider")
	})
}

// alreadyExistsKeyManagement wraps TestKeyManagement so CreateKey always returns
// a gRPC AlreadyExists error, while GetKey uses the pre-existing nativeID.
type alreadyExistsKeyManagement struct {
	*testplugins.TestKeyManagement

	nativeID string
}

func (m *alreadyExistsKeyManagement) CreateKey(
	_ context.Context,
	_ *keymanagement.CreateKeyRequest,
) (*keymanagement.CreateKeyResponse, error) {
	return nil, grpcstatus.Error(codes.AlreadyExists, "resource already exists")
}

func (m *alreadyExistsKeyManagement) GetKey(
	ctx context.Context,
	req *keymanagement.GetKeyRequest,
) (*keymanagement.GetKeyResponse, error) {
	// Redirect to the pre-existing key so GetKey succeeds for recovery.
	req.Parameters.KeyID = m.nativeID
	resp, err := m.TestKeyManagement.GetKey(ctx, req)
	if err != nil {
		return nil, err
	}
	// Ensure KeyID is set so recoverExistingProviderKey can assign the NativeID.
	resp.KeyID = m.nativeID
	return resp, nil
}

func TestSyncPendingCreationKeyEdgeCases(t *testing.T) {
	t.Run("skips sync when key no longer exists", func(t *testing.T) {
		km, _, ctx, _, _ := SetupKeyTest(t)

		// Use a UUID that was never persisted — simulates key deleted between enqueue and processing.
		err := km.SyncPendingCreationKey(ctx, uuid.New())

		require.NoError(t, err, "deleted key should be silently skipped")
	})

	t.Run("skips sync when key is not in PENDING_CREATION state", func(t *testing.T) {
		provider := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig, _ := SetupKeyTest(t, testplugins.WithKeyManagement(testplugins.Name, provider))

		// Seed an ENABLED key directly (e.g. already transitioned before the task ran).
		key := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStateENABLED, provider)

		err := km.SyncPendingCreationKey(ctx, key.ID)

		require.NoError(t, err, "non-PENDING_CREATION key should be a no-op")
	})
}

func TestEnqueuePendingStateSync(t *testing.T) {
	t.Run("enqueues task successfully when async client is set", func(t *testing.T) {
		mockClient := &async.MockClient{}
		provider := testplugins.NewTestKeyManagement(true, true)
		km, r, ctx, keyConfig := SetupKeyTestWithAsyncClient(t, mockClient, testplugins.WithKeyManagement(testplugins.Name, provider))

		key := createTestBYOKKey(t, r, ctx, keyConfig.ID, cmkapi.KeyStatePENDINGCREATION, provider)

		// Create via the manager: async client is set → should enqueue
		// We seed the keystore so provisioning is NOT needed, meaning Create won't enqueue.
		// Instead call Create without default keystore to trigger PENDING_CREATION path.
		newKey := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
		})
		// NeedsDefaultKeystoreProvisioning returns true (no stored config) → triggers enqueue.
		_, err := km.Create(ctx, newKey)
		require.NoError(t, err)

		_ = key // used to keep createTestBYOKKey reference
		assert.Equal(t, 1, mockClient.EnqueueCallCount, "one task should be enqueued")
		assert.Equal(t, config.TypePendingStateSync, mockClient.LastTask.Type())
	})

	t.Run("logs error and continues when enqueue fails", func(t *testing.T) {
		mockClient := &async.MockClient{Error: errMockAsyncUnavailable}
		km, _, ctx, keyConfig := SetupKeyTestWithAsyncClient(t, mockClient)

		newKey := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeBYOK
		})
		// Even with enqueue failure, Create should succeed (enqueue errors are non-fatal).
		_, err := km.Create(ctx, newKey)
		require.NoError(t, err)

		assert.Equal(t, 1, mockClient.EnqueueCallCount, "enqueue was attempted")
	})
}

// SetupKeyTestWithAsyncClient sets up a KeyManager with a real async client mock for testing
// enqueuePendingStateSync paths.
func SetupKeyTestWithAsyncClient(
	t *testing.T,
	asyncClient async.Client,
	opts ...testplugins.RegistryOption,
) (*manager.KeyManager, repo.Repo, context.Context, *model.KeyConfiguration) {
	t.Helper()

	db, tenants, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	svcRegistry := testutils.NewTestPlugins(opts...)
	cryptoCerts := []config.CryptoCert{
		{
			Name: "crypto-1",
			Subject: config.CryptoCertSubject{
				Locality:           []string{"Berlin"},
				OrganizationalUnit: []string{"OU1"},
				Organization:       []string{"TestOrg"},
				Country:            []string{"DE"},
				CommonNamePrefix:   "test_",
			},
			RootCA: "https://example.com/root.crt",
		},
	}
	cryptoCertsBytes, err := yaml.Marshal(cryptoCerts)
	require.NoError(t, err)

	cfg := &config.Config{
		Database: dbConf,
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(cryptoCertsBytes),
			},
		},
	}

	cmkAuditor := auditor.New(ctx, cfg)
	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	require.NoError(t, err)

	certManager := manager.NewCertificateManager(ctx, r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, certManager)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)

	km := manager.NewKeyManager(
		r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, eventFactory, cmkAuditor, asyncClient,
	)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		tenantDefaultCert,
		testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeRoleManagement
			c.CommonName = testutils.TestDefaultKeystoreCommonName
		}),
		testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeKeyManagement
			c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
		}),
		testutils.NewKeystore(func(_ *model.Keystore) {}),
	)

	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

	return km, r, ctx, keyConfig
}
