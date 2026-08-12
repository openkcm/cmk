package manager_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var (
	ksConfig            = testutils.NewKeystore(func(_ *model.Keystore) {})
	keystoreDefaultCert = testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeRoleManagement
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
	keystoreKeyMgmtCert = testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeKeyManagement
		c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
	})
)

func setupKeyVersionManager(t *testing.T) (*manager.KeyVersionManager, repo.Repo, string, uuid.UUID) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	tenant := tenants[0]

	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	svcRegistry := testutils.NewTestPlugins()
	cfg := config.Config{}

	certManager := manager.NewCertificateManager(
		ctx, r, svcRegistry,
		&config.Config{
			Certificates: config.Certificates{ValidityDays: config.MinCertificateValidityDays},
		})
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil)
	cmkAuditor := auditor.New(ctx, &cfg)

	kvm := manager.NewKeyVersionManager(
		r,
		svcRegistry,
		tenantConfigManager,
		certManager,
		cmkAuditor,
	)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		ksConfig,
		keystoreDefaultCert,
		keystoreKeyMgmtCert,
	)

	return kvm, r, tenant, keyConfig.ID
}

func TestKeyVersionManager_List(t *testing.T) {
	kvm, r, tenant, keyConfigID := setupKeyVersionManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should list key versions", func(t *testing.T) {
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
		testutils.CreateTestEntities(ctx, t, r, key)

		pagination := repo.Pagination{
			Skip:  constants.DefaultSkip,
			Top:   constants.DefaultTop,
			Count: true,
		}
		result, _, err := kvm.GetKeyVersions(ctx, key.ID, pagination)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result, len(key.KeyVersions))
	})

	t.Run("Should use created_at as tie-breaker when rotated_at is identical", func(t *testing.T) {
		keyID := uuid.New()

		sharedRotationTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfigID
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		_, err := kvm.CreateVersion(ctx, keyID, "version-1", &sharedRotationTime)
		require.NoError(t, err)

		_, err = kvm.CreateVersion(ctx, keyID, "version-2", &sharedRotationTime)
		require.NoError(t, err)

		version3, err := kvm.CreateVersion(ctx, keyID, "version-3", &sharedRotationTime)
		require.NoError(t, err)

		latest, err := kvm.GetLatestVersion(ctx, keyID)
		require.NoError(t, err)
		assert.NotNil(t, latest)

		pagination := repo.Pagination{
			Skip:  0,
			Top:   10,
			Count: true,
		}
		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, pagination)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
		assert.Len(t, allVersions, 3)

		assert.Equal(t, version3.ID, latest.ID, "Latest should be version3 (most recently created)")
		assert.Equal(t, "version-3", latest.NativeID)
		assert.Equal(t, "version-3", allVersions[0].NativeID)

		latest2, err := kvm.GetLatestVersion(ctx, keyID)
		require.NoError(t, err)
		assert.Equal(t, latest.ID, latest2.ID, "GetLatestVersion should be deterministic")

		allVersions2, _, err := kvm.GetKeyVersions(ctx, keyID, pagination)
		require.NoError(t, err)
		assert.Equal(t, allVersions[0].ID, allVersions2[0].ID, "GetKeyVersions ordering should be deterministic")
	})

	t.Run("Should handle concurrent version creation gracefully", func(t *testing.T) {
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfigID
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		nativeID := "concurrent-test-version-1"
		rotationTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

		version1, err := kvm.CreateVersion(ctx, keyID, nativeID, &rotationTime)
		require.NoError(t, err)
		assert.NotNil(t, version1)
		assert.Equal(t, nativeID, version1.NativeID)
		assert.Equal(t, keyID, version1.KeyID)

		version2, err := kvm.CreateVersion(ctx, keyID, nativeID, &rotationTime)
		assert.NoError(t, err, "Concurrent creation should not fail")
		assert.NotNil(t, version2)
		assert.Equal(t, nativeID, version2.NativeID)
		assert.Equal(t, keyID, version2.KeyID)

		assert.Equal(t, version1.ID, version2.ID, "Should return existing version on duplicate")

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   10,
			Count: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "Should have only one version")
		assert.Len(t, allVersions, 1)
		assert.Equal(t, version1.ID, allVersions[0].ID)
	})
}
