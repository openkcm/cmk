package manager_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	testRegionUSEast1 = "us-east-1"
)

func SetupKeyTest(t *testing.T) (
	*manager.KeyManager,
	repo.Repo,
	context.Context,
	*model.KeyConfiguration,
) {
	t.Helper()

	db, tenants, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	ps, psCfg := testutils.NewTestPlugins(
		testplugins.NewKeystoreOperator(),
	)

	cryptoCerts := []manager.ClientCertificate{
		{
			Name: "crypto-1",
			Subject: manager.ClientCertificateSubject{
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
		Plugins:  psCfg,
		Database: dbConf,
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(cryptoCertsBytes),
			},
		},
	}
	svcRegistry, err := cmkpluginregistry.New(ctx, cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)

	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil)
	certManager := manager.NewCertificateManager(ctx, r, svcRegistry,
		&config.Config{
			Certificates: config.Certificates{ValidityDays: config.MinCertificateValidityDays},
		})
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	require.NoError(t, err)

	km := manager.NewKeyManager(
		r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, eventFactory, cmkAuditor)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		tenantDefaultCert,
		keystoreDefaultCert,
		ksConfig,
	)

	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

	return km, r, ctx, keyConfig
}

func createTestSystemManagedKey(t *testing.T, km *manager.KeyManager, ctx context.Context, keyConfigID uuid.UUID) *model.Key {
	t.Helper()
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigID
	})

	createdKey, err := km.Create(ctx, key)
	require.NoError(t, err)

	return createdKey
}

func createTestHYOKKey(t *testing.T, km *manager.KeyManager, ctx context.Context, keyConfigID uuid.UUID) *model.Key {
	t.Helper()
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigID
		k.KeyType = constants.KeyTypeHYOK
		k.NativeID = ptr.PointTo("mock-key/11111111")
		k.ManagementAccessData = hyokInfo
		k.Provider = providerTest
	})

	createdKey, err := km.Create(ctx, key)
	require.NoError(t, err)

	return createdKey
}

func createTestBYOKKey(t *testing.T, r repo.Repo, ctx context.Context, keyConfigID uuid.UUID, state string) *model.Key {
	t.Helper()
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigID
		k.KeyType = constants.KeyTypeBYOK
		k.State = state
		k.NativeID = ptr.PointTo("arn:aws:kms:us-west-2:111122223333:alias/<alias-name>")
	})

	testutils.CreateTestEntities(ctx, t, r, key)

	return key
}

func TestCreate(t *testing.T) {
	km, r, ctx, keyConfig := SetupKeyTest(t)

	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	tests := []struct {
		name    string
		key     func() *model.Key
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid managed key creation",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
				})
			},
			wantErr: false,
		},
		{
			name: "Invalid provider",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = constants.KeyTypeHYOK
					k.NativeID = ptr.PointTo("mock-key/11111111")
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
					k.KeyType = constants.KeyTypeHYOK
					k.NativeID = ptr.PointTo("mock-key/11111111")
					k.ManagementAccessData = hyokInfo
					k.Provider = providerTest
				})
			},
			wantErr: false,
		},
		{
			name: "HYOK key creation wrong access data",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = constants.KeyTypeHYOK
					k.NativeID = ptr.PointTo("mock-key/11111111")
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
					k.KeyType = constants.KeyTypeHYOK
					k.NativeID = ptr.PointTo("invalid-key-id")
					k.ManagementAccessData = hyokInfo
					k.Provider = providerTest
				})
			},
			wantErr: true,
			errMsg:  "HYOK provider key not found",
		},
		{
			name: "ValidBYOKKeyCreation",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.KeyType = constants.KeyTypeBYOK
					k.State = string(cmkapi.KeyStatePENDINGIMPORT)
				})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.key()
			result, err := km.Create(ctx, key)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, key.ID, result.ID)
				assert.NotNil(t, result.NativeID)
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
		localCtx := testutils.InjectClientDataIntoContext(
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
	km, _, ctx, keyConfig := SetupKeyTest(t)

	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	t.Run("should add certificate subject to crypto access data when cert name matches", func(t *testing.T) {
		cryptoAccessData := model.KeyAccessData{
			"crypto-1": {"someKey": "someValue"},
		}
		cryptoBytes, err := json.Marshal(cryptoAccessData)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeHYOK
			k.NativeID = ptr.PointTo("mock-key/11111111")
			k.ManagementAccessData = hyokInfo
			k.Provider = providerTest
			k.CryptoAccessData = cryptoBytes
		})

		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)

		resultData := createdKey.GetCryptoAccessData()
		require.NotNil(t, resultData)
		require.Contains(t, resultData, "crypto-1")
		assert.Contains(t, resultData["crypto-1"], "certificateSubject")

		subject, ok := resultData["crypto-1"]["certificateSubject"].(string)
		require.True(t, ok)
		assert.Contains(t, subject, "OU=OU1/OU2")
		assert.Contains(t, subject, "O=TestOrg")
		assert.Contains(t, subject, "L=Berlin")
		assert.Contains(t, subject, "C=DE")
	})

	t.Run("should not add certificate subject when cert name does not match", func(t *testing.T) {
		cryptoAccessData := model.KeyAccessData{
			"non-existent-cert": {"someKey": "someValue"},
		}
		cryptoBytes, err := json.Marshal(cryptoAccessData)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeHYOK
			k.NativeID = ptr.PointTo("mock-key/11111111")
			k.ManagementAccessData = hyokInfo
			k.Provider = providerTest
			k.CryptoAccessData = cryptoBytes
		})

		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)

		resultData := createdKey.GetCryptoAccessData()
		require.NotNil(t, resultData)
		require.Contains(t, resultData, "non-existent-cert")
		assert.NotContains(t, resultData["non-existent-cert"], "certificateSubject")
	})

	t.Run("should handle HYOK key with no crypto access data", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeHYOK
			k.NativeID = ptr.PointTo("mock-key/11111111")
			k.ManagementAccessData = hyokInfo
			k.Provider = providerTest
		})

		createdKey, err := km.Create(ctx, key)
		require.NoError(t, err)
		assert.NotNil(t, createdKey)
	})
}

func TestSetFirstKeyPrimary(t *testing.T) {
	km, r, ctx, keyConfig := SetupKeyTest(t)

	t.Run("Should set first key as primary", func(t *testing.T) {
		createdKey1 := createTestSystemManagedKey(t, km, ctx, keyConfig.ID)
		assert.True(t, createdKey1.IsPrimary)

		createdKey2 := createTestSystemManagedKey(t, km, ctx, keyConfig.ID)
		assert.False(t, createdKey2.IsPrimary)

		resKeyConfig := &model.KeyConfiguration{ID: keyConfig.ID, AdminGroup: model.Group{ID: uuid.New()}}
		_, err := r.First(ctx, resKeyConfig, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, createdKey1.ID, *resKeyConfig.PrimaryKeyID)
	})
}

func TestEditableCryptoData(t *testing.T) {
	km, r, ctx, _ := SetupKeyTest(t)

	regionEditable := "region1"
	regionNonEditable := "region2"

	cryptoData, err := json.Marshal(model.KeyAccessData{
		regionEditable:    map[string]any{},
		regionNonEditable: map[string]any{},
	})
	require.NoError(t, err)

	t.Run("Should all be editable on non primary Key", func(t *testing.T) {
		kc := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

		sysFailed := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = ptr.PointTo(kc.ID)
			sys.Region = regionEditable
			sys.Status = cmkapi.SystemStatusFAILED
		})

		sysConnected := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = ptr.PointTo(kc.ID)
			sys.Region = regionNonEditable
			sys.Status = cmkapi.SystemStatusCONNECTED
		})

		key := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = false
			k.CryptoAccessData = cryptoData
			k.KeyConfigurationID = kc.ID
		})

		testutils.CreateTestEntities(ctx, t, r, kc, sysFailed, sysConnected, key)
		localCtx := testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{kc.AdminGroup.IAMIdentifier})

		key, err = km.Get(localCtx, key.ID)
		assert.NoError(t, err)

		assert.True(t, key.EditableRegions[regionEditable])
		assert.True(t, key.EditableRegions[regionNonEditable])
	})

	t.Run("Should be editable on pkey only on failed regions", func(t *testing.T) {
		kc := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

		sysFailed := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = ptr.PointTo(kc.ID)
			sys.Region = regionEditable
			sys.Status = cmkapi.SystemStatusFAILED
		})

		sysConnected := testutils.NewSystem(func(sys *model.System) {
			sys.KeyConfigurationID = ptr.PointTo(kc.ID)
			sys.Region = regionNonEditable
			sys.Status = cmkapi.SystemStatusCONNECTED
		})

		testutils.CreateTestEntities(ctx, t, r, kc, sysFailed, sysConnected)

		key := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = true
			k.CryptoAccessData = cryptoData
			k.KeyConfigurationID = kc.ID
		})
		localCtx := testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{kc.AdminGroup.IAMIdentifier})

		key, err = km.Create(localCtx, key)
		require.NoError(t, err)

		key, err = km.Get(localCtx, key.ID)
		assert.NoError(t, err)

		assert.True(t, key.EditableRegions[regionEditable])
		assert.False(t, key.EditableRegions[regionNonEditable])
	})
}

func TestGet(t *testing.T) {
	km, r, ctx, keyConfig := SetupKeyTest(t)

	createdKey := createTestSystemManagedKey(t, km, ctx, keyConfig.ID)
	hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)
	byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

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
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		gotKey, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), gotKey.State)
	})

	t.Run("HYOK key state syncs after provider disable", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		disableKey(t, km, ctx, hyokKey)

		key, err = km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateDISABLED), key.State)
		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
	})

	t.Run("hyok state syncs after provider disable", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

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
		assert.Equal(t, string(cmkapi.KeyStateDISABLED), key.State)
		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
	})

	t.Run("hyok sync delete", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		deleteKey(t, km, ctx, hyokKey)
		err = km.SyncHYOKKeys(ctx)
		assert.NoError(t, err)
		key, err = km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStatePENDINGDELETION), key.State)
	})

	t.Run("hyok sync delete/enable", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		deleteKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStatePENDINGDELETION))

		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateENABLED))
	})

	t.Run("hyok sync delete/disable", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		deleteKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStatePENDINGDELETION))

		disableKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateDISABLED))
	})

	t.Run("hyok state syncs on key deleted", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		key.NativeID = ptr.PointTo("invalid-key-id")
		_, err = r.Patch(ctx, key, *repo.NewQuery())
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateDELETED))
	})

	t.Run("hyok state syncs on auth change", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		key.ManagementAccessData = []byte("{\"invalid\": \"data\"}")
		_, err = r.Patch(ctx, key, *repo.NewQuery())
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateFORBIDDEN))
	})

	t.Run("hyok state disable twice then enable twice", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		hyokKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		key, err := km.Get(ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		disableKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateDISABLED))

		disableKey(t, km, ctx, hyokKey)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateDISABLED))

		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateENABLED))

		err = enableKey(t, km, ctx, hyokKey)
		assert.NoError(t, err)
		syncAndVerifyState(t, km, ctx, hyokKey, string(cmkapi.KeyStateENABLED))
	})
}

func syncAndVerifyState(t *testing.T, km *manager.KeyManager, ctx context.Context, hyokKey *model.Key, expectedState string) {
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
	km, r, ctx, keyConfig := SetupKeyTest(t)

	createTestSystemManagedKey(t, km, ctx, keyConfig.ID)
	createTestSystemManagedKey(t, km, ctx, keyConfig.ID)

	sys := testutils.NewSystem(func(sys *model.System) {
		sys.Status = cmkapi.SystemStatusFAILED
		sys.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
	})

	testutils.CreateTestEntities(ctx, t, r, sys)

	tests := []struct {
		name          string
		skip          int
		top           int
		keyConfigID   *uuid.UUID
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "List all keys",
			skip:          0,
			top:           10,
			keyConfigID:   nil,
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "List all keys from same keyConfig",
			skip:          0,
			top:           10,
			keyConfigID:   ptr.PointTo(keyConfig.ID),
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "List with pagination",
			skip:          0,
			top:           1,
			keyConfigID:   nil,
			expectedCount: 2,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := km.GetKeys(ctx, nil, repo.Pagination{Skip: tt.skip, Top: tt.top, Count: true})

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
	km, _, ctx, keyConfig := SetupKeyTest(t)
	createdKey := createTestSystemManagedKey(t, km, ctx, keyConfig.ID)

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
				Name:        ptr.PointTo("updated-name"),
				Description: ptr.PointTo("Updated description"),
			},
			wantErr: false,
		},
		{
			name:  "Disable key",
			keyID: createdKey.ID,
			keyPatch: cmkapi.KeyPatch{
				Enabled: ptr.PointTo(false),
			},
			wantErr: false,
		},
		{
			name:  "Enable key",
			keyID: createdKey.ID,
			keyPatch: cmkapi.KeyPatch{
				Enabled: ptr.PointTo(true),
			},
			wantErr: false,
		},
		{
			name:  "Non-existent key",
			keyID: uuid.New(),
			keyPatch: cmkapi.KeyPatch{
				Name: ptr.PointTo("new-name"),
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
					assert.Equal(t, *tt.keyPatch.Enabled, updatedKey.State == string(cmkapi.KeyStateENABLED))

					if *tt.keyPatch.Enabled {
						assert.Equal(t, string(cmkapi.KeyStateENABLED), updatedKey.State)
					} else {
						assert.Equal(t, string(cmkapi.KeyStateDISABLED), updatedKey.State)
					}
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	km, r, ctx, keyConfig := SetupKeyTest(t)

	createdKey := createTestSystemManagedKey(t, km, ctx, keyConfig.ID)
	createdPrimaryKey, err := km.Create(ctx, testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
		k.IsPrimary = true
	}))
	require.NoError(t, err)
	byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

	keyConfigWSystems := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfigWSystems.ID)
	})
	keyFailSystems := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigWSystems.ID
		k.IsPrimary = true
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

func TestUpdateKeyPrimary(t *testing.T) {
	t.Run("Should update primary key and exiting events", func(t *testing.T) {
		km, r, ctx, _ := SetupKeyTest(t)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		oldPrimaryKey := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = true
			k.KeyConfigurationID = keyConfig.ID
		})
		keyConfig.PrimaryKeyID = &oldPrimaryKey.ID

		sys := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})

		key := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = false
			k.KeyConfigurationID = keyConfig.ID
		})

		data := eventprocessor.SystemActionJobData{
			KeyIDTo: oldPrimaryKey.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		event := &model.Event{
			Identifier: uuid.NewString(),
			Type:       proto.TaskType_SYSTEM_SWITCH.String(),
			Data:       dataBytes,
		}

		testutils.CreateTestEntities(ctx, t, r, keyConfig, oldPrimaryKey, key, sys, event)
		localCtx := testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		key, err = km.UpdateKey(localCtx, key.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(true),
		})
		assert.NoError(t, err)
		assert.True(t, key.IsPrimary)

		resKeyConfig := &model.KeyConfiguration{ID: keyConfig.ID}
		_, err = r.First(localCtx, resKeyConfig, *repo.NewQuery())
		assert.NoError(t, err)

		assert.Equal(t, key.ID, *resKeyConfig.PrimaryKeyID)

		_, err = r.First(localCtx, event, *repo.NewQuery())
		assert.NoError(t, err)
		jobData, err := eventprocessor.GetSystemJobData(event)
		assert.NoError(t, err)
		assert.Equal(t, key.ID.String(), jobData.KeyIDTo)

		oldK1, err := km.Get(localCtx, oldPrimaryKey.ID)
		assert.NoError(t, err)
		assert.False(t, oldK1.IsPrimary)
	})

	t.Run("Should use old pkey on switch event when updating ", func(t *testing.T) {
		km, r, ctx, _ := SetupKeyTest(t)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		oldPrimaryKey := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = true
			k.KeyConfigurationID = keyConfig.ID
		})
		keyConfig.PrimaryKeyID = &oldPrimaryKey.ID

		sys := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})

		key := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = false
			k.KeyConfigurationID = keyConfig.ID
		})

		testutils.CreateTestEntities(ctx, t, r, keyConfig, oldPrimaryKey, key, sys)
		localCtx := testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		k, err := km.UpdateKey(localCtx, key.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(true),
		})
		assert.NoError(t, err)
		assert.True(t, k.IsPrimary)

		orbitalCtx := testutils.CreateCtxWithTenant("orbital")
		jobFromDB := &testutils.OrbitalJob{}
		_, err = r.First(
			orbitalCtx,
			jobFromDB,
			*repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where("external_id", sys.ID.String()),
				),
			),
		)
		assert.NoError(t, err)

		jobData := &eventprocessor.SystemActionJobData{}
		err = json.Unmarshal(jobFromDB.Data, jobData)
		assert.NoError(t, err)
		assert.Equal(t, oldPrimaryKey.ID.String(), jobData.KeyIDFrom)
	})

	t.Run("Should error on set primary on disabled key", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)

		key1 := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = false
			k.State = string(cmkapi.KeyStateDISABLED)
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctx, t, r, key1)
		_, err := km.UpdateKey(ctx, key1.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(true),
		})
		assert.ErrorIs(t, err, manager.ErrKeyIsNotEnabled)
	})

	t.Run("Should error on unmark primary on primary key", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)

		key1 := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = true
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctx, t, r, key1)
		_, err := km.UpdateKey(ctx, key1.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(false),
		})
		assert.ErrorIs(t, err, manager.ErrPrimaryKeyUnmark)
	})
}

func TestGetImportParams(t *testing.T) {
	cachedPublicKeyFromDB := "mock-public-key-from-database"
	fetchedPublicKeyFromProvider := "mock-public-key-from-provider"

	t.Run("Success_NilImportParams", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, fetchedPublicKeyFromProvider, got.PublicKeyPEM)
	})

	t.Run("Success_ImportParamsNotExpired", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = ptr.PointTo(time.Now().Add(24 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)
		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cachedPublicKeyFromDB, got.PublicKeyPEM)
	})

	t.Run("Success_NilExpires", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

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
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = ptr.PointTo(time.Now().Add(-1 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)
		got, err := km.GetImportParams(ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, fetchedPublicKeyFromProvider, got.PublicKeyPEM)
	})

	t.Run("Error_InvalidKeyType", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		sysKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)
		_, err := km.GetImportParams(ctx, sysKey.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyTypeForImportParams)
		assert.Contains(t, err.Error(), "key type")
	})

	t.Run("Error_InvalidKeyState", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)

		byokEnabledKey := testutils.NewKey(func(k *model.Key) {
			k.KeyType = string(cmkapi.KeyTypeBYOK)
			k.State = string(cmkapi.KeyStateENABLED)
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctx, t, r, byokEnabledKey)

		_, err := km.GetImportParams(ctx, byokEnabledKey.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyStateForImportParams)
		assert.Contains(t, err.Error(), "key state")
	})

	t.Run("Error_KeyNotFound", func(t *testing.T) {
		km, _, ctx, _ := SetupKeyTest(t)
		_, err := km.GetImportParams(ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestImportKeyMaterial(t *testing.T) {
	validMaterial := "dGVzdC1rZXktbWF0ZXJpYWw="

	t.Run("ImportParamsMissing", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		_, err := km.ImportKeyMaterial(ctx, byokKey.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrMissingOrExpiredImportParams)
		assert.Contains(t, err.Error(), "import parameters missing or expired")
	})

	t.Run("ImportParamsExpired", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		paramsJSON, err := json.Marshal(map[string]any{
			"providerParams": "test-provider-params",
		})
		assert.NoError(t, err)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.ProviderParameters = paramsJSON
			ip.Expires = ptr.PointTo(time.Now().Add(-1 * time.Hour))
		})
		testutils.CreateTestEntities(ctx, t, r, importParams)

		_, err = km.ImportKeyMaterial(ctx, byokKey.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrMissingOrExpiredImportParams)
		assert.Contains(t, err.Error(), "import parameters missing or expired")
	})

	t.Run("Success", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

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
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		_, err := km.ImportKeyMaterial(ctx, byokKey.ID, "")

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrEmptyKeyMaterial)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("InvalidBase64WrappedKeyMaterial", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

		_, err := km.ImportKeyMaterial(ctx, byokKey.ID, "not-base64")

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidBase64KeyMaterial)
		assert.Contains(t, err.Error(), "base64")
	})

	t.Run("InvalidKeyType", func(t *testing.T) {
		km, _, ctx, keyConfig := SetupKeyTest(t)
		sysKey := createTestHYOKKey(t, km, ctx, keyConfig.ID)

		_, err := km.ImportKeyMaterial(ctx, sysKey.ID, validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyTypeForImportKeyMaterial)
		assert.Contains(t, err.Error(), "key type")
	})

	t.Run("InvalidKeyState", func(t *testing.T) {
		km, r, ctx, keyConfig := SetupKeyTest(t)
		enabledBYOK := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStateENABLED))

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
		km, _, ctx, _ := SetupKeyTest(t)

		_, err := km.ImportKeyMaterial(ctx, uuid.New(), validMaterial)

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrGetKeyDB)
	})
}

func TestKeyRotationTime(t *testing.T) {
	// Known rotation time from keystore
	keystoreRotationTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	keystoreRotationTimeStr := keystoreRotationTime.Format(time.RFC3339)

	// Setup plugin with custom rotation time
	pluginOps := testplugins.NewKeystoreOperatorInstance()
	// Register the key in the plugin first
	pluginOps.HandleKeyRecord("test-native-id", testplugins.EnabledKeyStatus)
	pluginOps.SetKeyVersionInfo("test-native-id", "version-1", keystoreRotationTimeStr)

	// Use custom setup similar to SetupKeyTest but with our plugin instance
	db, tenants, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	ps, psCfg := testutils.NewTestPlugins(testplugins.NewKeystoreOperatorFromInstance(pluginOps))
	cryptoCerts := []manager.ClientCertificate{
		{
			Name: "crypto-1",
			Subject: manager.ClientCertificateSubject{
				CommonNamePrefix: "test_",
			},
			RootCA: "https://example.com/root.crt",
		},
	}
	cryptoCertsBytes, err := yaml.Marshal(cryptoCerts)
	require.NoError(t, err)

	cfg := &config.Config{
		Plugins:  psCfg,
		Database: dbConf,
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(cryptoCertsBytes),
			},
		},
	}
	svcRegistry, err := cmkpluginregistry.New(ctx, cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	require.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil)
	certManager := manager.NewCertificateManager(ctx, r, svcRegistry,
		&config.Config{
			Certificates: config.Certificates{ValidityDays: config.MinCertificateValidityDays},
		})
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)
	km := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor)

	// Create test data
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})
	keystoreDefaultCert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeKeystoreDefault
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
	ksConfig := testutils.NewKeystore(func(_ *model.Keystore) {})

	testutils.CreateTestEntities(ctx, t, r, keyConfig, tenantDefaultCert, keystoreDefaultCert, ksConfig)

	// Inject client data for auth
	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

	t.Run("Register HYOK key - should use rotation time from keystore", func(t *testing.T) {
		// Create HYOK key
		hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.Name = "test-hyok-key"
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeHYOK
			k.Algorithm = string(cmkapi.KeyAlgorithmAES256)
			k.Provider = testplugins.Name
			k.Region = testRegionUSEast1
			k.NativeID = ptr.PointTo("test-native-id")
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
				repo.NewCompositeKey().Where("key_id", createdKey.ID))),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		require.Len(t, versions, 1)

		// Verify rotation time matches keystore time (not current time)
		version := versions[0]
		assert.False(t, version.RotatedAt.IsZero(), "RotatedAt should be set")
		assert.Equal(t, keystoreRotationTime.Unix(), version.RotatedAt.Unix(),
			"RotatedAt should match keystore rotation time, not current time")
	})

	t.Run("Detect key rotation - should use rotation time from keystore", func(t *testing.T) {
		// Create initial key with version
		initialRotationTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
		require.NoError(t, err)

		// Register the key in the plugin with initial version
		pluginOps.HandleKeyRecord("test-native-id-rotate", testplugins.EnabledKeyStatus)
		pluginOps.SetKeyVersionInfo("test-native-id-rotate", "version-1", initialRotationTime.Format(time.RFC3339))

		key := testutils.NewKey(func(k *model.Key) {
			k.KeyType = constants.KeyTypeHYOK
			k.Provider = testplugins.Name
			k.NativeID = ptr.PointTo("test-native-id-rotate")
			k.KeyConfigurationID = keyConfig.ID
			k.ManagementAccessData = hyokInfo
			k.KeyVersions = []model.KeyVersion{
				{
					ID:        uuid.New(),
					NativeID:  "version-1",
					RotatedAt: initialRotationTime,
				},
			}
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		// Setup plugin to return new version with specific rotation time
		newVersionRotationTime := time.Date(2025, 6, 20, 14, 45, 0, 0, time.UTC)
		newVersionRotationTimeStr := newVersionRotationTime.Format(time.RFC3339)
		pluginOps.SetKeyVersionInfo("test-native-id-rotate", "version-2", newVersionRotationTimeStr)

		// Manually trigger sync for our key (use SyncHYOKKeys which syncs all)
		err = km.SyncHYOKKeys(ctx)
		require.NoError(t, err)

		// Fetch versions again
		versions, count, err := repo.ListAndCount(
			ctx, r, repo.Pagination{Skip: 0, Top: 10, Count: true},
			model.KeyVersion{},
			repo.NewQuery().Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("key_id", key.ID))),
		)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "Should have 2 versions after rotation")
		require.Len(t, versions, 2)

		// Find the new version
		var newVersion *model.KeyVersion
		for _, v := range versions {
			if v.NativeID == "version-2" {
				newVersion = v
				break
			}
		}
		require.NotNil(t, newVersion, "Should find the new version")

		// Verify rotation time matches keystore time for new version
		assert.False(t, newVersion.RotatedAt.IsZero(), "RotatedAt should be set")
		assert.Equal(t, newVersionRotationTime.Unix(), newVersion.RotatedAt.Unix(),
			"New version RotatedAt should match keystore rotation time")

		// Verify it's the rotation time from keystore, not current time
		now := time.Now().UTC()
		timeDiff := now.Sub(newVersion.RotatedAt)
		assert.Greater(t, timeDiff.Hours(), float64(24*30*6), // More than 6 months ago
			"RotatedAt should be the keystore time (2025-06-20), not current time")
	})

	t.Run("Fallback to current time when keystore doesn't provide rotation time", func(t *testing.T) {
		// Setup plugin without rotation time
		pluginOpsNoTime := testplugins.NewKeystoreOperatorInstance()
		// Register key but don't set rotation time (empty string)
		pluginOpsNoTime.HandleKeyRecord("test-native-id-no-time", testplugins.EnabledKeyStatus)
		pluginOpsNoTime.SetKeyVersionInfo("test-native-id-no-time", "version-1", "") // Empty rotation time

		ps2, psCfg2 := testutils.NewTestPlugins(testplugins.NewKeystoreOperatorFromInstance(pluginOpsNoTime))
		cfg2 := config.Config{Plugins: psCfg2}
		svcRegistry2, err := cmkpluginregistry.New(ctx, &cfg2, cmkpluginregistry.WithBuiltInPlugins(ps2))
		require.NoError(t, err)

		km2 := manager.NewKeyManager(r, svcRegistry2, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor)

		// Create HYOK key
		hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
		require.NoError(t, err)

		key := testutils.NewKey(func(k *model.Key) {
			k.Name = "test-hyok-no-time"
			k.KeyConfigurationID = keyConfig.ID
			k.KeyType = constants.KeyTypeHYOK
			k.Algorithm = string(cmkapi.KeyAlgorithmAES256)
			k.Provider = testplugins.Name
			k.Region = testRegionUSEast1
			k.NativeID = ptr.PointTo("test-native-id-no-time")
			k.ManagementAccessData = hyokInfo
		})

		beforeCreate := time.Now().UTC()
		createdKey, err := km2.Create(ctx, key)
		afterCreate := time.Now().UTC()

		require.NoError(t, err)
		require.NotNil(t, createdKey)

		// Fetch version
		versions, _, err := repo.ListAndCount(
			ctx, r, repo.Pagination{Skip: 0, Top: 10, Count: true},
			model.KeyVersion{},
			repo.NewQuery().Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where("key_id", createdKey.ID))),
		)
		require.NoError(t, err)
		require.Len(t, versions, 1)

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
	km, r, ctx, keyConfig := SetupKeyTest(t)

	// Setup test: Create a primary key with some systems
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	require.NoError(t, err)

	primaryKey := testutils.NewKey(func(k *model.Key) {
		k.Name = "primary-key"
		k.KeyConfigurationID = keyConfig.ID
		k.KeyType = constants.KeyTypeHYOK
		k.Algorithm = string(cmkapi.KeyAlgorithmAES256)
		k.Provider = testplugins.Name
		k.Region = testRegionUSEast1
		k.NativeID = ptr.PointTo("primary-key-native-id")
		k.ManagementAccessData = hyokInfo
		k.IsPrimary = true
	})

	nonPrimaryKey := testutils.NewKey(func(k *model.Key) {
		k.Name = "non-primary-key"
		k.KeyConfigurationID = keyConfig.ID
		k.KeyType = constants.KeyTypeHYOK
		k.Algorithm = string(cmkapi.KeyAlgorithmAES256)
		k.Provider = testplugins.Name
		k.Region = testRegionUSEast1
		k.NativeID = ptr.PointTo("non-primary-key-native-id")
		k.ManagementAccessData = hyokInfo
		k.IsPrimary = false
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
		err = km.ExportedHandleNewKeyVersion(ctx, primaryKey, &keymanagement.GetKeyResponse{
			LatestKeyVersionId: "new-version-id",
			RotationTime:       &rotationTime,
		}, &rotationTime)
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
		err = km.ExportedHandleNewKeyVersion(ctx, nonPrimaryKey, &keymanagement.GetKeyResponse{
			LatestKeyVersionId: "non-primary-new-version",
			RotationTime:       &rotationTime,
		}, &rotationTime)
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
			k.KeyType = constants.KeyTypeHYOK
			k.Provider = testplugins.Name
			k.Region = testRegionUSEast1
			k.NativeID = ptr.PointTo("empty-key-native-id")
			k.ManagementAccessData = hyokInfo
			k.IsPrimary = true
		})
		testutils.CreateTestEntities(ctx, t, r, emptyKey)

		rotationTime := time.Now().UTC()

		eventsBefore, err := countEvents(ctx, r, eventprocessor.JobTypeSystemKeyRotate.String())
		require.NoError(t, err)

		// Should not error even with no systems
		err = km.ExportedHandleNewKeyVersion(ctx, emptyKey, &keymanagement.GetKeyResponse{
			LatestKeyVersionId: "empty-key-new-version",
			RotationTime:       &rotationTime,
		}, &rotationTime)
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
			repo.NewCompositeKey().Where("type", eventType))),
	)
	if err != nil {
		return 0, err
	}
	return count, nil
}
