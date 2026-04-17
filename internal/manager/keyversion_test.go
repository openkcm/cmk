package manager_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
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

	ps, psCfg := testutils.NewTestPlugins(
		testplugins.NewKeystoreOperator(),
	)

	cfg := config.Config{Plugins: psCfg}
	svcRegistry, err := cmkpluginregistry.New(s.ctx, &cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	s.Require().NoError(err)

	certManager := manager.NewCertificateManager(
		s.ctx, s.r, svcRegistry,
		&config.Config{
			Certificates: config.Certificates{ValidityDays: config.MinCertificateValidityDays},
		})
	tenantConfigManager := manager.NewTenantConfigManager(s.r, svcRegistry, nil)
	cmkAuditor := auditor.New(s.ctx, &cfg)
	userManager := manager.NewUserManager(s.r, nil, cmkAuditor)
	tagManager := manager.NewTagManager(s.r)
	keyConfigManager := manager.NewKeyConfigManager(s.r, certManager, userManager, tagManager, cmkAuditor, &cfg)
	s.km = manager.NewKeyManager(s.r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor)
	s.kvm = manager.NewKeyVersionManager(
		s.r,
		svcRegistry,
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

func (s *KeyVersionManagerSuit) TestKeyVersionManager_List() {
	s.Run("Should list key versions", func() {
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyVersions = []model.KeyVersion{
				*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
					kv.KeyID = keyID
					kv.NativeID = "version-1"
				}),
				*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
					kv.KeyID = keyID
					kv.NativeID = "version-2"
				}),
			}
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)

		pagination := repo.Pagination{
			Skip:  constants.DefaultSkip,
			Top:   constants.DefaultTop,
			Count: true,
		}
		result, _, err := s.kvm.GetKeyVersions(s.ctx, key.ID, pagination)

		s.NoError(err)
		s.NotNil(result)
		s.Len(result, len(key.KeyVersions))
	})

	s.Run("Should use created_at as tie-breaker when rotated_at is identical", func() {
		// Create a key with multiple versions sharing the same rotated_at
		keyID := uuid.New()

		// All versions share the same rotation time (caller-supplied timestamp scenario)
		sharedRotationTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = s.keyConfigID
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)

		// Create versions with same rotated_at but different created_at
		// CreateTestEntities inserts them sequentially, so created_at will naturally differ
		_, err := s.kvm.CreateVersion(s.ctx, keyID, "version-1", &sharedRotationTime)
		s.NoError(err)

		_, err = s.kvm.CreateVersion(s.ctx, keyID, "version-2", &sharedRotationTime)
		s.NoError(err)

		version3, err := s.kvm.CreateVersion(s.ctx, keyID, "version-3", &sharedRotationTime)
		s.NoError(err)

		// Get latest version - should be deterministic based on created_at
		latest, err := s.kvm.GetLatestVersion(s.ctx, keyID)
		s.NoError(err)
		s.NotNil(latest)

		// Get all versions - should be ordered by rotated_at DESC, created_at DESC
		pagination := repo.Pagination{
			Skip:  0,
			Top:   10,
			Count: true,
		}
		allVersions, count, err := s.kvm.GetKeyVersions(s.ctx, keyID, pagination)
		s.NoError(err)
		s.Equal(3, count)
		s.Len(allVersions, 3)

		// Latest version should be the one with most recent created_at
		// (version3 was created last)
		s.Equal(version3.ID, latest.ID, "Latest should be version3 (most recently created)")
		s.Equal("version-3", latest.NativeID)
		s.Equal("version-3", allVersions[0].NativeID)

		// Verify ordering is deterministic: call again and should get same result
		latest2, err := s.kvm.GetLatestVersion(s.ctx, keyID)
		s.NoError(err)
		s.Equal(latest.ID, latest2.ID, "GetLatestVersion should be deterministic")

		allVersions2, _, err := s.kvm.GetKeyVersions(s.ctx, keyID, pagination)
		s.NoError(err)
		s.Equal(allVersions[0].ID, allVersions2[0].ID, "GetKeyVersions ordering should be deterministic")
	})

	s.Run("Should handle concurrent version creation gracefully", func() {
		// Create a key for testing concurrent version creation
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = s.keyConfigID
		})
		testutils.CreateTestEntities(s.ctx, s.T(), s.r, key)

		// Same version parameters (simulating concurrent refresh detecting same version)
		nativeID := "concurrent-test-version-1"
		rotationTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

		// First creation should succeed
		version1, err := s.kvm.CreateVersion(s.ctx, keyID, nativeID, &rotationTime)
		s.NoError(err)
		s.NotNil(version1)
		s.Equal(nativeID, version1.NativeID)
		s.Equal(keyID, version1.KeyID)

		// Second creation with same (key_id, native_id) should handle unique constraint
		// and return the existing version instead of failing
		version2, err := s.kvm.CreateVersion(s.ctx, keyID, nativeID, &rotationTime)
		s.NoError(err, "Concurrent creation should not fail")
		s.NotNil(version2)
		s.Equal(nativeID, version2.NativeID)
		s.Equal(keyID, version2.KeyID)

		// Should return the existing version (same ID)
		s.Equal(version1.ID, version2.ID, "Should return existing version on duplicate")

		// Verify only one version exists in database
		allVersions, count, err := s.kvm.GetKeyVersions(s.ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   10,
			Count: true,
		})
		s.NoError(err)
		s.Equal(1, count, "Should have only one version")
		s.Len(allVersions, 1)
		s.Equal(version1.ID, allVersions[0].ID)
	})
}
