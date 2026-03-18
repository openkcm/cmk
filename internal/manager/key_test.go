package manager_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
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

	cfg := &config.Config{
		Plugins:  psCfg,
		Database: dbConf,
	}
	svcRegistry, err := cmkpluginregistry.New(ctx, cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	assert.NoError(t, err)

	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil)
	certManager := manager.NewCertificateManager(ctx, r, svcRegistry,
		&config.Config{
			Certificates: config.Certificates{ValidityDays: config.MinCertificateValidityDays},
		})
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)

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

func TestSetFirstKeyPrimary(t *testing.T) {
	km, r, ctx, keyConfig := SetupKeyTest(t)

	t.Run("Should set first key as primary", func(t *testing.T) {
		createdKey1 := createTestSystemManagedKey(t, km, ctx, keyConfig.ID)

		_ = createTestSystemManagedKey(t, km, ctx, keyConfig.ID)

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
	}))
	require.NoError(t, err)
	byokKey := createTestBYOKKey(t, r, ctx, keyConfig.ID, string(cmkapi.KeyStatePENDINGIMPORT))

	keyID := uuid.New()
	keyConfigWSystems := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(keyID)
	})
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfigWSystems.ID)
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
