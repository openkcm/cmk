package manager_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/auditor"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/constants"
	eventprocessor "github.com/openkcm/cmk-core/internal/event-processor"
	"github.com/openkcm/cmk-core/internal/grpc/catalog"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	"github.com/openkcm/cmk-core/utils/ptr"
)

//nolint:containedctx
type KeyManagerSuite struct {
	suite.Suite

	km          *manager.KeyManager
	db          *multitenancy.DB
	repo        repo.Repo
	ctx         context.Context
	cleanupFunc func()
	keyConfigID uuid.UUID
	tenant      string
}

func TestKeyManagerSuite(t *testing.T) {
	suite.Run(t, new(KeyManagerSuite))
}

func (s *KeyManagerSuite) SetupSuite() {
	db, tenants := testutils.NewTestDB(s.T(), testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Key{},
			&model.System{},
			&model.KeyVersion{},
			&model.TenantConfig{},
			&model.Certificate{},
			&model.KeyConfigurationTag{},
			&model.ImportParams{},
			&model.KeystoreConfiguration{},
		},
	})
	s.db = db
	s.tenant = tenants[0]
	s.ctx = testutils.CreateCtxWithTenant(s.tenant)

	dbRepo := sql.NewRepository(s.db)
	s.repo = dbRepo

	cfg := config.Config{
		Plugins: testutils.SetupMockPlugins(
			testutils.KeyStorePlugin,
			testutils.KeystoreProviderPlugin,
			testutils.CertIssuer,
		),
		Database: testutils.TestDB,
	}
	ctlg, err := catalog.New(s.ctx, cfg)
	s.Require().NoError(err)

	tenantConfigManager := manager.NewTenantConfigManager(dbRepo, ctlg)
	certManager := manager.NewCertificateManager(s.ctx, dbRepo, ctlg,
		&config.Certificates{ValidityDays: config.MinCertificateValidityDays})
	keyConfigManager := manager.NewKeyConfigManager(dbRepo, certManager, &cfg)

	reconciler, err := eventprocessor.NewCryptoReconciler(s.ctx, &cfg, dbRepo, ctlg)
	s.Require().NoError(err)

	cmkAuditor := auditor.New(s.ctx, &cfg)
	s.km = manager.NewKeyManager(
		dbRepo, ctlg, tenantConfigManager, keyConfigManager, certManager, reconciler, cmkAuditor)

	// Create test key configuration once for all tests
	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.Name = "test-config"
	})
	s.keyConfigID = keyConfig.ID
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})

	testutils.CreateTestEntities(
		s.ctx,
		s.T(),
		s.repo,
		keyConfig,
		tenantDefaultCert,
		keystoreDefaultCert,
		ksConfig,
	)
}

func (s *KeyManagerSuite) SetupTest() {
	// Clean up the database before each test
	testutils.RunTestQuery(
		s.db,
		s.tenant,
		"DELETE FROM key_versions",
		"DELETE FROM import_params",
		"DELETE FROM keys",
	)
}

func (s *KeyManagerSuite) TearDownSuite() {
	// Final cleanup
	testutils.RunTestQuery(
		s.db,
		s.tenant,
		"DELETE FROM key_versions",
		"DELETE FROM import_params",
		"DELETE FROM keys",
	)

	if s.cleanupFunc != nil {
		s.cleanupFunc()
	}
}

func (s *KeyManagerSuite) createTestSystemManagedKey(name string) *model.Key {
	key := &model.Key{
		ID:                 uuid.New(),
		KeyConfigurationID: s.keyConfigID,
		KeyType:            constants.KeyTypeSystemManaged,
		Name:               name,
		Description:        "Test key description",
		Algorithm:          "RSA3072",
		Provider:           providerTest,
		Region:             "us-east-1",
		State:              string(cmkapi.KeyStateENABLED),
	}

	createdKey, err := s.km.Create(s.ctx, key)
	s.Require().NoError(err)

	return createdKey
}

func (s *KeyManagerSuite) createTestHYOKKey(name string) *model.Key {
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	s.Require().NoError(err)
	key := &model.Key{
		ID:                   uuid.New(),
		KeyConfigurationID:   s.keyConfigID,
		KeyType:              constants.KeyTypeHYOK,
		NativeID:             ptr.PointTo("mock-key/11111111"),
		Name:                 name,
		Description:          "Test key description",
		Algorithm:            "AES256",
		Provider:             providerTest,
		Region:               "us-east-1",
		State:                string(cmkapi.KeyStateENABLED),
		ManagementAccessData: hyokInfo,
	}

	createdKey, err := s.km.Create(s.ctx, key)
	s.Require().NoError(err)

	return createdKey
}

func (s *KeyManagerSuite) createTestBYOKKey(name, state string) *model.Key {
	key := &model.Key{
		ID:                 uuid.New(),
		KeyConfigurationID: s.keyConfigID,
		KeyType:            constants.KeyTypeBYOK,
		Name:               name,
		Description:        "Test key description",
		Algorithm:          "RSA3072",
		Provider:           providerTest,
		Region:             "us-east-1",
		State:              state,
		NativeID:           ptr.PointTo("arn:aws:kms:us-west-2:111122223333:alias/<alias-name>"),
	}

	testutils.CreateTestEntities(s.ctx, s.T(), s.repo, key)

	return key
}

// Standalone test for GetPluginAlgorithm as it doesn't need suite setup
func TestGetPluginAlgorithm(t *testing.T) {
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
			result := manager.GetPluginAlgorithm(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func (s *KeyManagerSuite) TestGetOrInitProvider() {
	key := s.createTestSystemManagedKey("test")
	s.Run("Valid provider", func() {
		provider, err := s.km.GetOrInitProvider(s.ctx, key)
		s.NoError(err)
		s.NotNil(provider)
	})

	s.Run("Invalid provider", func() {
		key.Provider = "GCP"
		key.KeyType = constants.KeyTypeHYOK
		provider, err := s.km.GetOrInitProvider(s.ctx, key)
		s.Error(err)
		s.Nil(provider)
		s.ErrorIs(err, manager.ErrPluginNotFound)
		s.EqualError(err, "plugin not found: GCP")
	})
}

//nolint:funlen
func (s *KeyManagerSuite) TestCreate() {
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	s.Require().NoError(err)

	tests := []struct {
		name    string
		key     func() *model.Key
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid managed key creation",
			key: func() *model.Key {
				return &model.Key{
					ID:                 uuid.New(),
					KeyConfigurationID: s.keyConfigID,
					Name:               "test-key",
					Description:        "Test key description",
					Algorithm:          "RSA3072",
					KeyType:            constants.KeyTypeSystemManaged,
					Provider:           providerTest,
					Region:             "us-east-1",
					State:              string(cmkapi.KeyStateENABLED),
				}
			},
			wantErr: false,
		},
		{
			name: "Invalid provider",
			key: func() *model.Key {
				return &model.Key{
					ID:                 uuid.New(),
					KeyConfigurationID: s.keyConfigID,
					Name:               "test-key",
					Algorithm:          "RSA3072",
					KeyType:            constants.KeyTypeSystemManaged,
					Provider:           "INVALID",
					Region:             "us-east-1",
				}
			},
			wantErr: true,
		},
		{
			name: "Valid HYOK key creation",
			key: func() *model.Key {
				return &model.Key{
					ID:                   uuid.New(),
					KeyConfigurationID:   s.keyConfigID,
					KeyType:              constants.KeyTypeHYOK,
					NativeID:             ptr.PointTo("mock-key/11111111"),
					Name:                 "test-key-2",
					Description:          "Test key description",
					Algorithm:            "AES256",
					Provider:             providerTest,
					Region:               "us-east-1",
					State:                string(cmkapi.KeyStateENABLED),
					ManagementAccessData: hyokInfo,
				}
			},
			wantErr: false,
		},
		{
			name: "HYOK key creation wrong access data",
			key: func() *model.Key {
				return &model.Key{
					ID:                   uuid.New(),
					KeyConfigurationID:   s.keyConfigID,
					KeyType:              constants.KeyTypeHYOK,
					NativeID:             ptr.PointTo("mock-key/11111111"),
					Name:                 "test-key-3",
					Description:          "Test key description",
					Algorithm:            "AES256",
					Provider:             providerTest,
					Region:               "us-east-1",
					State:                string(cmkapi.KeyStateENABLED),
					ManagementAccessData: []byte("{\"invalid\": \"data\"}"),
				}
			},
			wantErr: true,
			errMsg:  "failed to authenticate with the keystore provider: Invalid account information",
		},
		{
			name: "HYOK key creation key not found",
			key: func() *model.Key {
				return &model.Key{
					ID:                   uuid.New(),
					KeyConfigurationID:   s.keyConfigID,
					KeyType:              constants.KeyTypeHYOK,
					NativeID:             ptr.PointTo("invalid-key-id"),
					Name:                 "test-key-2",
					Description:          "Test key description",
					Algorithm:            "AES256",
					Provider:             providerTest,
					Region:               "us-east-1",
					State:                string(cmkapi.KeyStateENABLED),
					ManagementAccessData: hyokInfo,
				}
			},
			wantErr: true,
			errMsg:  "HYOK provider key not found",
		},
		{
			name: "ValidBYOKKeyCreation",
			key: func() *model.Key {
				return &model.Key{
					ID:                 uuid.New(),
					KeyConfigurationID: s.keyConfigID,
					KeyType:            constants.KeyTypeBYOK,
					Name:               "byok-test-key",
					Description:        "Test key description",
					Algorithm:          "RSA3072",
					Provider:           providerTest,
					Region:             "us-east-1",
					State:              string(cmkapi.KeyStatePENDINGIMPORT),
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			key := tt.key()
			result, err := s.km.Create(s.ctx, key)

			if tt.wantErr {
				s.Error(err)
				s.Nil(result)
				s.Contains(err.Error(), tt.errMsg)
			} else {
				s.NoError(err)
				s.NotNil(result)
				s.Equal(key.ID, result.ID)
				s.NotNil(result.NativeID)
			}
		})
	}
}

func (s *KeyManagerSuite) TestSetFirstKeyPrimary() {
	t := s.T()

	t.Run("Should set first key as primary", func(t *testing.T) {
		createdKey1 := s.createTestSystemManagedKey("get-test-key-1")
		assert.True(t, createdKey1.IsPrimary)

		createdKey2 := s.createTestSystemManagedKey("get-test-key-2")
		assert.False(t, createdKey2.IsPrimary)
		// Verify that the first key is set as primary in the key configuration
		resKeyConfig := &model.KeyConfiguration{ID: s.keyConfigID, AdminGroup: model.Group{ID: uuid.New()}}
		_, err := s.repo.First(s.ctx, resKeyConfig, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, createdKey1.ID, *resKeyConfig.PrimaryKeyID)
	})
}

func (s *KeyManagerSuite) TestGet() {
	createdKey := s.createTestSystemManagedKey("get-test-key")
	hyokKey := s.createTestHYOKKey("get-test-hyok-key")
	byokKey := s.createTestBYOKKey("get-test-byok-key", string(cmkapi.KeyStatePENDINGIMPORT))

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
		s.Run(tt.name, func() {
			result, err := s.km.Get(s.ctx, tt.keyID)

			if tt.wantErr {
				s.Error(err)
				s.Nil(result)
			} else {
				s.NoError(err)
				s.NotNil(result)
				s.Equal(tt.keyID, result.ID)
			}
		})
	}
}

func (s *KeyManagerSuite) TestHYOKSync() {
	t := s.T()
	hyokKey := s.createTestHYOKKey("get-test-hyok-key")

	t.Run("HYOK key state is enabled after creation", func(t *testing.T) {
		gotKey, err := s.km.Get(s.ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), gotKey.State)
	})

	t.Run("HYOK key state syncs after provider disable", func(t *testing.T) {
		key, err := s.km.Get(s.ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		_ = s.disableKey(t, hyokKey)

		key, err = s.km.Get(s.ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateDISABLED), key.State)
		err = s.enableKey(t, hyokKey)
		assert.NoError(t, err)
	})

	t.Run("SyncHYOKkeys state syncs after provider disable", func(t *testing.T) {
		key, err := s.km.Get(s.ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateENABLED), key.State)

		provider, err := s.km.GetOrInitProvider(s.ctx, hyokKey)
		assert.NoError(t, err)
		_, err = provider.Client.DisableKey(s.ctx, &keystoreopv1.DisableKeyRequest{
			Parameters: &keystoreopv1.RequestParameters{
				KeyId:  *hyokKey.NativeID,
				Config: provider.Config,
			},
		})
		assert.NoError(t, err)
		err = s.km.SyncHYOKKeys(s.ctx)
		assert.NoError(t, err)
		// Verify that the key state is updated after sync
		key, err = s.km.Get(s.ctx, hyokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateDISABLED), key.State)
		err = s.enableKey(t, hyokKey)
		assert.NoError(t, err)
	})
}

func (s *KeyManagerSuite) disableKey(t *testing.T, hyokKey *model.Key) error {
	t.Helper()

	provider, err := s.km.GetOrInitProvider(s.ctx, hyokKey)
	assert.NoError(t, err)
	_, err = provider.Client.DisableKey(s.ctx, &keystoreopv1.DisableKeyRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId:  *hyokKey.NativeID,
			Config: provider.Config,
		},
	})
	assert.NoError(t, err)

	return err
}

func (s *KeyManagerSuite) enableKey(t *testing.T, hyokKey *model.Key) error {
	t.Helper()

	provider, err := s.km.GetOrInitProvider(s.ctx, hyokKey)
	assert.NoError(t, err)
	_, err = provider.Client.EnableKey(s.ctx, &keystoreopv1.EnableKeyRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId:  *hyokKey.NativeID,
			Config: provider.Config,
		},
	})
	assert.NoError(t, err)

	return err
}

func (s *KeyManagerSuite) TestList() {
	// Create test keys
	s.createTestSystemManagedKey("list-test-key-1")
	s.createTestSystemManagedKey("list-test-key-2")

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
			keyConfigID:   ptr.PointTo(s.keyConfigID),
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "List with pagination",
			skip:          0,
			top:           1,
			keyConfigID:   nil,
			expectedCount: 2, // Total count should still be 2
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			results, total, err := s.km.GetKeys(s.ctx, nil, tt.skip, tt.top)

			if tt.wantErr {
				s.Error(err)
				s.Nil(results)
			} else {
				s.NoError(err)
				s.NotNil(results)
				s.Equal(tt.expectedCount, total)
				s.LessOrEqual(len(results), tt.top)
			}
		})
	}
}

func (s *KeyManagerSuite) TestUpdate() {
	createdKey := s.createTestSystemManagedKey("update-test-key")

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
		s.Run(tt.name, func() {
			_, err := s.km.UpdateKey(s.ctx, tt.keyID, tt.keyPatch)

			if tt.wantErr {
				s.Error(err)
			} else {
				s.verifyUpdatedKey(err, tt)
			}
		})
	}
}

func (s *KeyManagerSuite) verifyUpdatedKey(err error, tt struct {
	name     string
	keyID    uuid.UUID
	keyPatch cmkapi.KeyPatch
	wantErr  bool
},
) {
	s.NoError(err)
	updatedKey, err := s.km.Get(s.ctx, tt.keyID)
	s.NoError(err)

	if tt.keyPatch.Name != nil {
		s.Equal(*tt.keyPatch.Name, updatedKey.Name)
	}

	if tt.keyPatch.Description != nil {
		s.Equal(*tt.keyPatch.Description, updatedKey.Description)
	}

	if tt.keyPatch.Enabled != nil {
		s.Equal(*tt.keyPatch.Enabled, updatedKey.State == string(cmkapi.KeyStateENABLED))

		if *tt.keyPatch.Enabled {
			s.Equal(string(cmkapi.KeyStateENABLED), updatedKey.State)
		} else {
			s.Equal(string(cmkapi.KeyStateDISABLED), updatedKey.State)
		}
	}
}

//nolint:funlen
func (s *KeyManagerSuite) TestDelete() {
	createdKey := s.createTestSystemManagedKey("delete-test-key")
	createdPrimaryKey, err := s.km.Create(s.ctx, &model.Key{
		ID:                 uuid.New(),
		Name:               uuid.NewString(),
		KeyType:            constants.KeyTypeSystemManaged,
		IsPrimary:          true,
		KeyConfigurationID: s.keyConfigID,
	})
	s.Require().NoError(err)
	byokKey := s.createTestBYOKKey("get-test-byok-key", string(cmkapi.KeyStatePENDINGIMPORT))

	keyConfigWSystems := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	sys := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfigWSystems.ID)
	})
	keyFailSystems := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigWSystems.ID
		k.IsPrimary = true
	})

	testutils.CreateTestEntities(s.ctx, s.T(), s.repo, keyConfigWSystems, sys, keyFailSystems)

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
		s.Run(tt.name, func() {
			err := s.km.Delete(s.ctx, tt.keyID)

			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				_, err := s.km.Get(s.ctx, tt.keyID)
				s.Error(err)
			}
		})
	}
}

func (s *KeyManagerSuite) TestUpdateVersion() {
	createdKey := s.createTestSystemManagedKey("update-version-test-key")

	tests := []struct {
		name    string
		keyID   uuid.UUID
		version int
		wantErr bool
	}{
		{
			name:    "Update Version - SUCCESS",
			keyID:   createdKey.ID,
			version: 3,
			wantErr: false,
		},
		{
			name:    "Update non-existent key - ERROR",
			keyID:   uuid.New(),
			version: 3,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := s.km.UpdateVersion(s.ctx, tt.keyID, tt.version)

			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				key, err := s.km.Get(s.ctx, tt.keyID)
				s.NoError(err)
				s.Equal(tt.version, key.Version().Version)
			}
		})
	}
}

//nolint:funlen
func (s *KeyManagerSuite) TestUpdateKeyPrimary() {
	t := s.T()

	t.Run("Should update primary key", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		oldPrimaryKey := testutils.NewKey(func(k *model.Key) {
			k.Name = uuid.NewString()
			k.IsPrimary = true
			k.KeyConfigurationID = keyConfig.ID
		})
		keyConfig.PrimaryKeyID = &oldPrimaryKey.ID

		key := testutils.NewKey(func(k *model.Key) {
			k.Name = uuid.NewString()
			k.IsPrimary = false
			k.KeyConfigurationID = keyConfig.ID
		})

		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, keyConfig, oldPrimaryKey, key)

		k, err := s.km.UpdateKey(s.ctx, key.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(true),
		})
		assert.NoError(t, err)
		assert.True(t, k.IsPrimary)

		resKeyConfig := &model.KeyConfiguration{ID: keyConfig.ID}
		_, err = s.repo.First(s.ctx, resKeyConfig, *repo.NewQuery())
		assert.NoError(t, err)

		assert.Equal(t, key.ID, *resKeyConfig.PrimaryKeyID)

		oldK1, err := s.km.Get(s.ctx, oldPrimaryKey.ID)
		assert.NoError(t, err)
		assert.False(t, oldK1.IsPrimary)
	})

	t.Run("Should error on set primary on disabled key", func(t *testing.T) {
		key1 := testutils.NewKey(func(k *model.Key) {
			k.Name = uuid.NewString()
			k.IsPrimary = false
			k.State = string(cmkapi.KeyStateDISABLED)
			k.KeyConfigurationID = uuid.New()
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, key1)
		_, err := s.km.UpdateKey(s.ctx, key1.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(true),
		})
		assert.ErrorIs(t, err, manager.ErrKeyIsNotEnabled)
	})

	t.Run("Should error on unmark primary on primary key", func(t *testing.T) {
		key1 := testutils.NewKey(func(k *model.Key) {
			k.Name = uuid.NewString()
			k.IsPrimary = true
			k.KeyConfigurationID = uuid.New()
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, key1)
		_, err := s.km.UpdateKey(s.ctx, key1.ID, cmkapi.KeyPatch{
			IsPrimary: ptr.PointTo(false),
		})
		assert.ErrorIs(t, err, manager.ErrPrimaryKeyUnmark)
	})
}

func (s *KeyManagerSuite) createFreshBYOKKey() *model.Key {
	testutils.RunTestQuery(
		s.db,
		s.tenant,
		"DELETE FROM import_params",
		"DELETE FROM keys",
	)

	return s.createTestBYOKKey("byok-importparams", string(cmkapi.KeyStatePENDINGIMPORT))
}

func (s *KeyManagerSuite) createEnabledBYOKKey() *model.Key {
	testutils.RunTestQuery(
		s.db,
		s.tenant,
		"DELETE FROM import_params",
		"DELETE FROM keys",
	)

	byokEnabledKey := testutils.NewKey(func(k *model.Key) {
		k.Name = "enabled-byok-importparams"
		k.KeyType = string(cmkapi.KeyTypeBYOK)
		k.State = string(cmkapi.KeyStateENABLED)
		k.KeyConfigurationID = s.keyConfigID
	})
	testutils.CreateTestEntities(s.ctx, s.T(), s.repo, byokEnabledKey)

	return byokEnabledKey
}

//nolint:funlen
func (s *KeyManagerSuite) TestGetImportParams() {
	cachedPublicKeyFromDB := "mock-public-key-from-database"
	fetchedPublicKeyFromProvider := "mock-public-key-from-provider"

	t := s.T()

	t.Run("Success_NilImportParams", func(t *testing.T) {
		byokKey := s.createFreshBYOKKey()
		got, err := s.km.GetImportParams(s.ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, fetchedPublicKeyFromProvider, got.PublicKeyPEM)
	})

	t.Run("Success_ImportParamsNotExpired", func(t *testing.T) {
		byokKey := s.createFreshBYOKKey()
		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = ptr.PointTo(time.Now().Add(24 * time.Hour))
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, importParams)
		got, err := s.km.GetImportParams(s.ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cachedPublicKeyFromDB, got.PublicKeyPEM)
	})

	t.Run("Success_NilExpires", func(t *testing.T) {
		byokKey := s.createFreshBYOKKey()
		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = nil
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, importParams)
		got, err := s.km.GetImportParams(s.ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, cachedPublicKeyFromDB, got.PublicKeyPEM)
	})

	t.Run("Success_ImportParamsExpired", func(t *testing.T) {
		byokKey := s.createFreshBYOKKey()
		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.PublicKeyPEM = cachedPublicKeyFromDB
			ip.Expires = ptr.PointTo(time.Now().Add(-1 * time.Hour))
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, importParams)
		got, err := s.km.GetImportParams(s.ctx, byokKey.ID)
		assert.NoError(t, err)
		assert.Equal(t, fetchedPublicKeyFromProvider, got.PublicKeyPEM)
	})

	t.Run("Error_InvalidKeyType", func(t *testing.T) {
		sysKey := s.createTestSystemManagedKey("sys-key")
		_, err := s.km.GetImportParams(s.ctx, sysKey.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyTypeForImportParams)
		assert.Contains(t, err.Error(), "key type")
	})

	t.Run("Error_InvalidKeyState", func(t *testing.T) {
		byokKey := s.createEnabledBYOKKey()
		_, err := s.km.GetImportParams(s.ctx, byokKey.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyStateForImportParams)
		assert.Contains(t, err.Error(), "key state")
	})

	t.Run("Error_KeyNotFound", func(t *testing.T) {
		_, err := s.km.GetImportParams(s.ctx, uuid.New())
		assert.Error(t, err)
	})
}

//nolint:funlen
func (s *KeyManagerSuite) TestImportKeyMaterial() {
	t := s.T()

	byokKey := s.createTestBYOKKey("byok-import", string(cmkapi.KeyStatePENDINGIMPORT))
	validMaterial := "dGVzdC1rZXktbWF0ZXJpYWw="

	paramsJSON, err := json.Marshal(map[string]any{
		"providerParams": "test-provider-params",
	})
	assert.NoError(t, err)

	s.Run("ImportParamsMissing", func() {
		// Prepare
		testutils.RunTestQuery(
			s.db,
			s.tenant,
			"DELETE FROM import_params",
		)

		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, byokKey.ID, validMaterial)

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrMissingOrExpiredImportParams)
		s.Contains(err.Error(), "import parameters missing or expired")
	})

	s.Run("ImportParamsExpired", func() {
		// Prepare
		testutils.RunTestQuery(
			s.db,
			s.tenant,
			"DELETE FROM import_params",
		)

		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.ProviderParameters = paramsJSON
			ip.Expires = ptr.PointTo(time.Now().Add(-1 * time.Hour))
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, importParams)

		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, byokKey.ID, validMaterial)

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrMissingOrExpiredImportParams)
		s.Contains(err.Error(), "import parameters missing or expired")
	})

	s.Run("Success", func() {
		// Prepare
		testutils.RunTestQuery(
			s.db,
			s.tenant,
			"DELETE FROM import_params",
		)
		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = byokKey.ID
			ip.ProviderParameters = paramsJSON
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, importParams)

		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, byokKey.ID, validMaterial)

		// Assert
		s.NoError(err)
	})

	s.Run("EmptyWrappedKeyMaterial", func() {
		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, byokKey.ID, "")

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrEmptyKeyMaterial)
		s.Contains(err.Error(), "empty")
	})

	s.Run("InvalidBase64WrappedKeyMaterial", func() {
		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, byokKey.ID, "not-base64")

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrInvalidBase64KeyMaterial)
		s.Contains(err.Error(), "base64")
	})

	s.Run("InvalidKeyType", func() {
		// Prepare
		sysKey := s.createTestSystemManagedKey("sys-key")

		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, sysKey.ID, validMaterial)

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrInvalidKeyTypeForImportKeyMaterial)
		s.Contains(err.Error(), "key type")
	})

	s.Run("InvalidKeyState", func() {
		// Prepare
		enabledBYOK := s.createTestBYOKKey("byok-enabled", string(cmkapi.KeyStateENABLED))
		importParams := testutils.NewImportParams(func(ip *model.ImportParams) {
			ip.KeyID = enabledBYOK.ID
			ip.ProviderParameters = paramsJSON
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.repo, importParams)

		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, enabledBYOK.ID, validMaterial)

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrInvalidKeyStateForImportKeyMaterial)
		s.Contains(err.Error(), "key state")
	})

	s.Run("KeyNotFound", func() {
		// Act
		_, err := s.km.ImportKeyMaterial(s.ctx, uuid.New(), validMaterial)

		// Assert
		s.Error(err)
		s.ErrorIs(err, manager.ErrGetKeyDB)
	})
}

func (s *KeyManagerSuite) TestCreateKeystore() {
	provider, createdKeystoreConfig, err := s.km.CreateKeystore(s.ctx)
	s.NoError(err)

	s.NotNil(createdKeystoreConfig)
	s.Equal(providerTest, provider)
	s.Equal("test-uuid", createdKeystoreConfig["locality"])
	s.Equal("default.kms.test", createdKeystoreConfig["commonName"])
}

func (s *KeyManagerSuite) TestFillKeystorePool() {
	err := s.km.FillKeystorePool(s.ctx, 2)
	s.NoError(err)

	// Verify that keystore pool has been filled
	keystoreConfigs := &[]model.KeystoreConfiguration{}
	count, err := s.repo.List(s.ctx, model.KeystoreConfiguration{}, keystoreConfigs, *repo.NewQuery())
	s.NoError(err)

	s.Equal(2, count)
}
