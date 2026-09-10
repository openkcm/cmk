package manager_test

import (
	"fmt"
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
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
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
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
	cmkAuditor := auditor.New(ctx, &cfg)

	// Create a test landscape config with default limits
	landscapeConfig := &config.Landscape{
		Name:   "test",
		Region: "test-region",
		MaxKeyVersions: map[string]int{
			"AWS":      5,
			"GCP":      5,
			"FORTANIX": 5,
		},
	}

	kvm := manager.NewKeyVersionManager(
		r,
		svcRegistry,
		tenantConfigManager,
		certManager,
		cmkAuditor,
		landscapeConfig,
	)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keyConfig,
		testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeRoleManagement
			c.CommonName = testutils.TestDefaultKeystoreCommonName
		}),
		testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeKeyManagement
			c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
		}),
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

func TestUpdateVersions(t *testing.T) {
	kvm, r, tenant, keyConfigID := setupKeyVersionManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should create multiple versions", func(t *testing.T) {
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfigID
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		creationTime1 := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC).UTC()
		creationTime2 := time.Date(2024, 1, 11, 10, 0, 0, 0, time.UTC).UTC()

		versions := []keymanagement.KeyVersion{
			{ID: "v1", CreationTime: &creationTime1},
			{ID: "v2", CreationTime: &creationTime2},
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  constants.DefaultSkip,
			Top:   constants.DefaultTop,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Len(t, allVersions, 2)

		assert.Equal(t, "v2", allVersions[0].NativeID)
		assert.Equal(t, "v1", allVersions[1].NativeID)
		assert.Equal(t, creationTime2, allVersions[0].RotatedAt.UTC())
		assert.Equal(t, creationTime1, allVersions[1].RotatedAt.UTC())
	})

	t.Run("Should update version if existing", func(t *testing.T) {
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfigID
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		creationTime := time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC)
		versions := []keymanagement.KeyVersion{
			{ID: "v1", CreationTime: &creationTime},
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)

		updatedTime := time.Date(2025, 2, 1, 10, 0, 0, 0, time.UTC)
		updatedVersions := []keymanagement.KeyVersion{
			{ID: "v1", CreationTime: &updatedTime},
		}

		err = kvm.UpdateVersions(ctx, keyID, updatedVersions)
		require.NoError(t, err)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  constants.DefaultSkip,
			Top:   constants.DefaultTop,
			Count: true,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Len(t, allVersions, 1)
		assert.Equal(t, "v1", allVersions[0].NativeID)
		assert.Equal(t, updatedTime, allVersions[0].RotatedAt.UTC())
	})

	t.Run("Should create version with current time if empty", func(t *testing.T) {
		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfigID
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		before := time.Now().UTC().Truncate(time.Microsecond)
		versions := []keymanagement.KeyVersion{
			{ID: "v1"},
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)
		after := time.Now().UTC().Truncate(time.Microsecond)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  constants.DefaultSkip,
			Top:   constants.DefaultTop,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Len(t, allVersions, 1)
		assert.Equal(t, "v1", allVersions[0].NativeID)
		assert.False(t, allVersions[0].RotatedAt.Before(before), "RotatedAt should be >= before")
		assert.False(t, allVersions[0].RotatedAt.After(after), "RotatedAt should be <= after")
	})
}

func TestVersionEviction(t *testing.T) {
	t.Run("Should evict oldest versions when limit exceeded", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		landscapeConfig := &config.Landscape{
			Name:   "test",
			Region: "test-region",
			MaxKeyVersions: map[string]int{
				"AWS": 5,
			},
		}

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, landscapeConfig,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "AWS"
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		var versions []keymanagement.KeyVersion
		for i := 1; i <= 10; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			versions = append(versions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 5, count, "Should keep only 5 most recent versions")
		assert.Len(t, allVersions, 5)

		assert.Equal(t, "v10", allVersions[0].NativeID, "Most recent version should be v10")
		assert.Equal(t, "v9", allVersions[1].NativeID)
		assert.Equal(t, "v8", allVersions[2].NativeID)
		assert.Equal(t, "v7", allVersions[3].NativeID)
		assert.Equal(t, "v6", allVersions[4].NativeID, "Oldest kept version should be v6")
	})

	t.Run("Should not evict when under limit", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		landscapeConfig := &config.Landscape{
			Name:   "test",
			Region: "test-region",
			MaxKeyVersions: map[string]int{
				"AWS": 5,
			},
		}

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, landscapeConfig,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "AWS"
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		var versions []keymanagement.KeyVersion
		for i := 1; i <= 3; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			versions = append(versions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Should keep all 3 versions")
		assert.Len(t, allVersions, 3)
	})

	t.Run("Should not evict when unlimited (-1)", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		landscapeConfig := &config.Landscape{
			Name:   "test",
			Region: "test-region",
			MaxKeyVersions: map[string]int{
				"FORTANIX": -1, // Unlimited
			},
		}

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, landscapeConfig,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "FORTANIX"
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		var versions []keymanagement.KeyVersion
		for i := 1; i <= 20; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			versions = append(versions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 20, count, "Should keep all 20 versions with unlimited config")
		assert.Len(t, allVersions, 20)
	})

	t.Run("Should enforce limits independently per provider", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		landscapeConfig := &config.Landscape{
			Name:   "test",
			Region: "test-region",
			MaxKeyVersions: map[string]int{
				"AWS": 5,
				"GCP": 3,
			},
		}

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, landscapeConfig,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		awsKeyID := uuid.New()
		awsKey := testutils.NewKey(func(k *model.Key) {
			k.ID = awsKeyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "AWS"
		})
		testutils.CreateTestEntities(ctx, t, r, awsKey)

		var awsVersions []keymanagement.KeyVersion
		for i := 1; i <= 10; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			awsVersions = append(awsVersions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("aws-v%d", i),
				CreationTime: &creationTime,
			})
		}
		err := kvm.UpdateVersions(ctx, awsKeyID, awsVersions)
		require.NoError(t, err)

		gcpKeyID := uuid.New()
		gcpKey := testutils.NewKey(func(k *model.Key) {
			k.ID = gcpKeyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "GCP"
		})
		testutils.CreateTestEntities(ctx, t, r, gcpKey)

		var gcpVersions []keymanagement.KeyVersion
		for i := 1; i <= 10; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			gcpVersions = append(gcpVersions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("gcp-v%d", i),
				CreationTime: &creationTime,
			})
		}
		err = kvm.UpdateVersions(ctx, gcpKeyID, gcpVersions)
		require.NoError(t, err)

		awsAllVersions, awsCount, err := kvm.GetKeyVersions(ctx, awsKeyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 5, awsCount, "AWS key should have 5 versions")
		assert.Len(t, awsAllVersions, 5)

		gcpAllVersions, gcpCount, err := kvm.GetKeyVersions(ctx, gcpKeyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, gcpCount, "GCP key should have 3 versions")
		assert.Len(t, gcpAllVersions, 3)
	})

	t.Run("Should keep primary version (most recent)", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		landscapeConfig := &config.Landscape{
			Name:   "test",
			Region: "test-region",
			MaxKeyVersions: map[string]int{
				"AWS": 3,
			},
		}

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, landscapeConfig,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "AWS"
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		var versions []keymanagement.KeyVersion
		for i := 1; i <= 6; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			versions = append(versions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err)

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Should keep 3 most recent versions")
		assert.Len(t, allVersions, 3)

		primaryVersion, err := kvm.GetLatestVersion(ctx, keyID)
		require.NoError(t, err)
		assert.Equal(t, "v6", primaryVersion.NativeID, "Primary version should be v6 (most recent)")
		assert.Equal(t, "v6", allVersions[0].NativeID, "Primary should be first in list")
		assert.Equal(t, primaryVersion.ID, allVersions[0].ID, "Primary should match latest")
	})

	t.Run("Should restore previously evicted version on re-addition", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		landscapeConfig := &config.Landscape{
			Name:   "test",
			Region: "test-region",
			MaxKeyVersions: map[string]int{
				"AWS": 3,
			},
		}

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, landscapeConfig,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "AWS"
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		var initialVersions []keymanagement.KeyVersion
		for i := 1; i <= 5; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			initialVersions = append(initialVersions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err := kvm.UpdateVersions(ctx, keyID, initialVersions)
		require.NoError(t, err)

		versionsAfterEviction, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Should have evicted to 3 versions")

		foundV1 := false
		for _, v := range versionsAfterEviction {
			if v.NativeID == "v1" {
				foundV1 = true
			}
		}
		assert.False(t, foundV1, "v1 should have been evicted")

		var reAddVersions []keymanagement.KeyVersion
		for i := 1; i <= 5; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			reAddVersions = append(reAddVersions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err = kvm.UpdateVersions(ctx, keyID, reAddVersions)
		require.NoError(t, err)

		versionsAfterReAdd, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Should still have 3 versions after re-add")

		foundV1AfterReAdd := false
		for _, v := range versionsAfterReAdd {
			if v.NativeID == "v1" {
				foundV1AfterReAdd = true
			}
		}
		assert.False(t, foundV1AfterReAdd, "v1 should be evicted again as it's still the oldest")
		assert.Equal(t, "v5", versionsAfterReAdd[0].NativeID, "v5 should be most recent")
		assert.Equal(t, "v4", versionsAfterReAdd[1].NativeID)
		assert.Equal(t, "v3", versionsAfterReAdd[2].NativeID)
	})

	t.Run("Should handle nil landscape config gracefully", func(t *testing.T) {
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
		tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, nil, nil)
		cmkAuditor := auditor.New(ctx, &cfg)

		kvm := manager.NewKeyVersionManager(
			r, svcRegistry, tenantConfigManager, certManager, cmkAuditor, nil,
		)

		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig,
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeRoleManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName
			}),
			testutils.NewCertificate(func(c *model.Certificate) {
				c.Purpose = model.CertificatePurposeKeyManagement
				c.CommonName = testutils.TestDefaultKeystoreCommonName + "-key-mgmt"
			}),
		)

		keyID := uuid.New()
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
			k.Provider = "AWS"
		})
		testutils.CreateTestEntities(ctx, t, r, key)

		var versions []keymanagement.KeyVersion
		for i := 1; i <= 10; i++ {
			creationTime := time.Date(2024, 1, i, 10, 0, 0, 0, time.UTC)
			versions = append(versions, keymanagement.KeyVersion{
				ID:           fmt.Sprintf("v%d", i),
				CreationTime: &creationTime,
			})
		}

		err := kvm.UpdateVersions(ctx, keyID, versions)
		require.NoError(t, err, "Should not crash with nil config")

		allVersions, count, err := kvm.GetKeyVersions(ctx, keyID, repo.Pagination{
			Skip:  0,
			Top:   100,
			Count: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 10, count, "Should keep all versions when config is nil")
		assert.Len(t, allVersions, 10)
	})
}
