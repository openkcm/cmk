package manager_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var (
	ksConfig            = testutils.NewKeystore(func(_ *model.Keystore) {})
	keystoreDefaultCert = testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeKeystoreDefault
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
)

//nolint:containedctx
type KeyVersionManagerSuit struct {
	suite.Suite

	km          *manager.KeyManager
	kvm         *manager.KeyVersionManager
	r           repo.Repo
	ctx         context.Context
	keyConfigID uuid.UUID
	tenant      string
}

func TestKeyVersionManagerSuit(t *testing.T) {
	suite.Run(t, new(KeyVersionManagerSuit))
}

func (s *KeyVersionManagerSuit) SetupSuite() {
	db, tenants, _ := testutils.NewTestDB(s.T(), testutils.TestDBConfig{})
	s.tenant = tenants[0]

	s.ctx = testutils.CreateCtxWithTenant(s.tenant)
	s.r = sql.NewRepository(db)

	cfg := config.Config{Plugins: testutils.SetupMockPlugins(testutils.KeyStorePlugin)}
	ctlg, err := catalog.New(s.ctx, &cfg)
	s.Require().NoError(err)

	tenantConfigManager := manager.NewTenantConfigManager(s.r, ctlg)
	certManager := manager.NewCertificateManager(
		s.ctx, s.r, ctlg, &config.Certificates{ValidityDays: config.MinCertificateValidityDays})
	cmkAuditor := auditor.New(s.ctx, &cfg)
	userManager := manager.NewUserManager(s.r, cmkAuditor)
	tagManager := manager.NewTagManager(s.r)
	keyConfigManager := manager.NewKeyConfigManager(s.r, certManager, userManager, tagManager, cmkAuditor, &cfg)
	s.km = manager.NewKeyManager(s.r, ctlg, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor)
	s.kvm = manager.NewKeyVersionManager(
		s.r,
		ctlg,
		tenantConfigManager,
		certManager,
		cmkAuditor,
	)

	// Create test key configuration once for all tests
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	testutils.CreateTestEntities(
		s.ctx,
		s.T(),
		s.r,
		keyConfig,
		ksConfig,
		keystoreDefaultCert,
	)
	s.keyConfigID = keyConfig.ID
}

func (s *KeyVersionManagerSuit) TestKeyVersionManager_AddKeyVersion() {
	s.Run("Should add key version", func() {
		keyVersion := testutils.NewKeyVersion(func(_ *model.KeyVersion) {})
		key := testutils.NewKey(func(_ *model.Key) {})

		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key, keyVersion)

		resultKeyVersion, err := s.kvm.AddKeyVersion(s.ctx, *key, key.NativeID)

		s.NoError(err)
		s.NotNil(resultKeyVersion)
		s.Equal(keyVersion.Version, resultKeyVersion.Version)
		oldKeyVersions, _, err := s.kvm.GetKeyVersions(s.ctx, key.ID, constants.DefaultSkip, constants.DefaultTop)
		s.NoError(err)

		for _, keyVersion := range oldKeyVersions {
			if keyVersion.ExternalID == resultKeyVersion.ExternalID {
				s.True(keyVersion.IsPrimary)
			} else {
				s.False(keyVersion.IsPrimary)
			}
		}
	})
}

func (s *KeyVersionManagerSuit) TestKeyVersionManager_CreateKeyVersion() {
	s.Run("Should error on non existing key", func() {
		key := testutils.NewKey(func(k *model.Key) { k.KeyConfigurationID = s.keyConfigID })
		_, err := s.kvm.CreateKeyVersion(s.ctx, key.ID, key.NativeID)
		s.ErrorIs(err, manager.ErrGetKeyDB)
	})
	s.Run("Should error on HYOK without nativeID", func() {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = s.keyConfigID
			k.KeyType = constants.KeyTypeHYOK
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)
		_, err := s.kvm.CreateKeyVersion(s.ctx, key.ID, key.NativeID)
		s.ErrorIs(err, manager.ErrNoBodyForCustomerHeldDB)
	})
	s.Run("Should create key version", func() {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = s.keyConfigID
			k.KeyType = providerTest
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)
		_, err := s.kvm.CreateKeyVersion(s.ctx, key.ID, key.NativeID)
		s.NoError(err)
	})
	s.Run("Should not create BYOK key version", func() {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = s.keyConfigID
			k.KeyType = constants.KeyTypeBYOK
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)

		_, err := s.kvm.CreateKeyVersion(s.ctx, key.ID, key.NativeID)
		s.ErrorIs(err, manager.ErrRotateBYOKKey)
	})
}

func (s *KeyVersionManagerSuit) TestKeyVersionManager_List() {
	s.Run("Should list key versions", func() {
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyVersions = []model.KeyVersion{
				*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
					kv.Version = 1
					kv.IsPrimary = false
					kv.Key.ID = keyID
					kv.KeyID = keyID
				}),
				*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
					kv.Version = 2
					kv.Key.ID = keyID
					kv.KeyID = keyID
				}),
			}
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)

		result, _, err := s.kvm.GetKeyVersions(s.ctx, key.ID, constants.DefaultSkip, constants.DefaultTop)

		s.NoError(err)
		s.NotNil(result)
		s.Len(result, len(key.KeyVersions))
	})
}

func (s *KeyVersionManagerSuit) TestKeyVersionManager_GetByKeyIDAndByNumber() {
	tests := []struct {
		name             string
		key              func() *model.Key
		keyVersions      func() []*model.KeyVersion
		keyVersionNumber string
		expectedErr      bool
	}{
		{
			name: "KeyVersionManager_GetByKeyIDAndByNumber_Latest_SUCCESS",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = s.keyConfigID
				})
			},
			keyVersions: func() []*model.KeyVersion {
				return []*model.KeyVersion{
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = false
						version.Version = 1
					}),
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = true
						version.Version = 2
					}),
				}
			},
			keyVersionNumber: "latest",
			expectedErr:      false,
		},
		{
			name: "KeyVersionManager_GetByKeyIDAndByNumber_Latest_ERROR",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = s.keyConfigID
				})
			},
			keyVersions: func() []*model.KeyVersion {
				return []*model.KeyVersion{}
			},
			keyVersionNumber: "latest",
			expectedErr:      true,
		},
		{
			name: "KeyVersionManager_GetByKeyIDAndByNumber_2_SUCCESS",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = s.keyConfigID
				})
			},
			keyVersions: func() []*model.KeyVersion {
				return []*model.KeyVersion{
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = false
						version.Version = 1
					}),
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = true
						version.Version = 2
					}),
				}
			},
			keyVersionNumber: "1",
			expectedErr:      false,
		},
		{
			name: "KeyVersionManager_GetByKeyIDAndByNumber_Invalid_ERROR",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = s.keyConfigID
				})
			},
			keyVersions: func() []*model.KeyVersion {
				return []*model.KeyVersion{
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = false
						version.Version = 1
					}),
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = true
						version.Version = 2
					}),
				}
			},
			keyVersionNumber: "invalid",
			expectedErr:      true,
		},
		{
			name: "KeyVersionManager_GetByKeyIDAndByNumber_NoKeyVersion_ERROR",
			key: func() *model.Key {
				return testutils.NewKey(func(k *model.Key) {
					k.KeyConfigurationID = s.keyConfigID
				})
			},
			keyVersions: func() []*model.KeyVersion {
				return []*model.KeyVersion{
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = false
						version.Version = 1
					}),
					testutils.NewKeyVersion(func(version *model.KeyVersion) {
						version.IsPrimary = true
						version.Version = 2
					}),
				}
			},
			keyVersionNumber: "10",
			expectedErr:      true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			key := tt.key()
			keyVersions := tt.keyVersions()

			var expectedVersion *model.KeyVersion

			err := s.r.Create(s.ctx, key)
			s.NoError(err)

			for _, keyVersion := range keyVersions {
				keyVersion.Key = *key
				err := s.r.Create(s.ctx, keyVersion)
				s.NoError(err)

				if !tt.expectedErr {
					if tt.keyVersionNumber == "latest" && keyVersion.IsPrimary == true {
						expectedVersion = keyVersion
					} else if tt.keyVersionNumber != "latest" {
						numberVersion, err := strconv.Atoi(tt.keyVersionNumber)
						s.NoError(err)

						if numberVersion == keyVersion.Version {
							expectedVersion = keyVersion
						}
					}
				}
			}

			result, err := s.kvm.GetByKeyIDAndByNumber(s.ctx, key.ID, tt.keyVersionNumber)

			if tt.expectedErr {
				s.Error(err)
				s.Nil(result)
			} else {
				s.NoError(err)
				s.NotNil(result)
				s.Equal(expectedVersion.IsPrimary, result.IsPrimary)
				s.Equal(expectedVersion.Version, result.Version)
				s.Equal(expectedVersion.ExternalID, result.ExternalID)
			}
		})
	}
}
