package manager_test

import (
	"crypto/x509/pkix"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/crypto"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	TestCertURL       = "https://aia.pki.co.test.com/aia/TEST%20Cloud%20Root%20CA.crt"
	cryptoSubject     = "CryptoCert"
	testAdminGroupIAM = "KMS_test_admin_group"
	adminGroup        = testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = testAdminGroupIAM
		g.Role = constants.KeyAdminRole
	})

	keyConfigWithAdminGroup = testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
		kc.AdminGroupID = adminGroup.ID
		kc.AdminGroup = *adminGroup
	})
	CreatorName = "bob@"
	CreatorID   = uuid.NewString()
)

func setupCfg(tb testing.TB) config.Config {
	tb.Helper()

	cryptoCerts := map[string]testutils.CryptoCert{
		"crypto-1": {
			Subject: cryptoSubject,
			RootCA:  TestCertURL,
		},
	}
	bytes, err := json.Marshal(cryptoCerts)
	assert.NoError(tb, err)

	return config.Config{
		Certificates: config.Certificates{
			RootCertURL:  TestCertURL,
			ValidityDays: config.MinCertificateValidityDays,
		},
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(bytes),
			},
		},
	}
}

func SetupKeyConfigManager(t *testing.T) (*manager.KeyConfigManager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	cfg := setupCfg(t)
	ctlg, err := catalog.New(t.Context(), &cfg)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), &cfg)

	dbRepository := sql.NewRepository(db)
	certManager := manager.NewCertificateManager(t.Context(), dbRepository, ctlg, &cfg.Certificates)
	userManager := manager.NewUserManager(dbRepository, cmkAuditor)
	tagManager := manager.NewTagManager(dbRepository)
	m := manager.NewKeyConfigManager(dbRepository, certManager, userManager, tagManager, cmkAuditor, &cfg)

	return m, db, tenants[0]
}

func TestNewKeyConfigManager(t *testing.T) {
	t.Run("Should create key config manager", func(t *testing.T) {
		m, _, _ := SetupKeyConfigManager(t)
		assert.NotNil(t, m)
	})
}

func TestGetKeyConfigurations(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	expected := []*model.KeyConfiguration{
		testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
			c.AdminGroup = *adminGroup
		}),
		testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
			c.AdminGroup = *adminGroup
		}),
	}

	for _, i := range expected {
		testutils.CreateTestEntities(ctx, t, r, i)
	}

	t.Run("Should get key configuration - IAM filter", func(t *testing.T) {
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAdminGroupIAM, "some_other_group"},
		)
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		actual, total, err := m.GetKeyConfigurations(ctxWithGroups, filter)
		assert.NoError(t, err)
		assert.Equal(t, len(expected), total)

		slices.SortFunc(expected, func(a, b *model.KeyConfiguration) int {
			return strings.Compare(a.ID.String(), b.ID.String())
		})

		for i := range actual {
			assert.Equal(t, expected[i].ID, actual[i].ID)
			assert.Equal(t, expected[i].Name, actual[i].Name)
		}
	})

	t.Run("Should get key configuration - Auditor read", func(t *testing.T) {
		testAuditorGroupIAM := "KMS_test_auditor_group"
		auditorGroup := testutils.NewGroup(func(g *model.Group) {
			g.IAMIdentifier = testAuditorGroupIAM
			g.Role = constants.TenantAuditorRole
		})

		ctx := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAuditorGroupIAM, "some_other_group"},
		)
		testutils.CreateTestEntities(ctx, t, r, auditorGroup)
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		actual, total, err := m.GetKeyConfigurations(ctx, filter)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, total)
		assert.NotEmpty(t, actual)
	})

	t.Run("Should get 0 key configuration - no access", func(t *testing.T) {
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{"group-no-access", "some_other_group"},
		)
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		_, total, err := m.GetKeyConfigurations(ctxWithGroups, filter)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
	})

	t.Run("Should get 0 key configuration - empty IAMGroups", func(t *testing.T) {
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{},
		)
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		_, total, err := m.GetKeyConfigurations(ctxWithGroups, filter)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
	})

	t.Run("Should get 1 key configuration - adminGroup2", func(t *testing.T) {
		adminGroupName2 := "KMS_admin_group_2"
		adminGroup2 := testutils.NewGroup(func(g *model.Group) {
			g.IAMIdentifier = adminGroupName2
			g.Role = constants.KeyAdminRole
		})
		keyConfig2 := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.AdminGroupID = adminGroup2.ID
			kc.AdminGroup = *adminGroup2
		})
		testutils.CreateTestEntities(ctx, t, r, adminGroup2, keyConfig2)

		// Create context with user's IAM groups including only adminGroup2
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{adminGroupName2, "some_other_group"},
		)
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		_, total, err := m.GetKeyConfigurations(ctxWithGroups, filter)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)
	})

	t.Run("Should err getting key configuration", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		_, _, err := m.GetKeyConfigurations(t.Context(), filter)
		assert.Error(t, err)
	})

	t.Run("Should get user keyconfig count", func(t *testing.T) {
		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

		groupA := testutils.NewGroup(func(_ *model.Group) {})
		groupB := testutils.NewGroup(func(_ *model.Group) {})
		testutils.CreateTestEntities(ctx, t, r, groupA, groupB)
		kcCount := 10
		for i := range 2 {
			var g *model.Group
			if i%2 == 1 {
				g = groupA
			} else {
				g = groupB
			}

			for range kcCount {
				kc := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
					kc.AdminGroup = *g
					kc.AdminGroupID = g.ID
				})
				testutils.CreateTestEntities(ctx, t, r, kc)
				for range 2 {
					k := testutils.NewKey(func(k *model.Key) {
						k.KeyConfigurationID = kc.ID
					})
					testutils.CreateTestEntities(ctx, t, r, k)
				}
			}
		}

		ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{groupA.IAMIdentifier})
		_, count, err := m.GetKeyConfigurations(ctx, manager.KeyConfigFilter{})
		assert.NoError(t, err)
		assert.Equal(t, kcCount, count)
	})
}

func TestTotalSystemAndKey(t *testing.T) {
	t.Run("Should get keyconfig with two keys and one system", func(t *testing.T) {
		m, db, tenant := SetupKeyConfigManager(t)
		assert.NotNil(t, m)
		r := sql.NewRepository(db)

		group := testutils.NewGroup(func(_ *model.Group) {})

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.Name = uuid.NewString()
			c.AdminGroupID = group.ID
		})

		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
		ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		sys := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.NewString(),
			KeyConfigurationID: &keyConfig.ID,
		}

		key1 := &model.Key{
			ID:                 uuid.New(),
			Name:               uuid.NewString(),
			KeyConfigurationID: keyConfig.ID,
			IsPrimary:          false,
		}

		key2 := &model.Key{
			ID:                 uuid.New(),
			Name:               uuid.NewString(),
			KeyConfigurationID: keyConfig.ID,
			IsPrimary:          false,
		}

		testutils.CreateTestEntities(ctx, t, r, group, keyConfig, sys, key1, key2)
		k, err := m.GetKeyConfigurationByID(ctx, keyConfig.ID)
		assert.NoError(t, err)
		assert.Equal(t, 2, k.TotalKeys)
		assert.Equal(t, 1, k.TotalSystems)
	})

	t.Run("Should get no entries on deleted keyconfig with items referencing it", func(t *testing.T) {
		m, db, tenant := SetupKeyConfigManager(t)
		assert.NotNil(t, m)
		r := sql.NewRepository(db)

		group := testutils.NewGroup(func(_ *model.Group) {})

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.Name = uuid.NewString()
			c.AdminGroupID = group.ID
		})

		sys := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.NewString(),
			KeyConfigurationID: &keyConfig.ID,
		}

		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
		ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		testutils.CreateTestEntities(ctx, t, r, group, keyConfig, sys)

		k, err := m.GetKeyConfigurationByID(ctx, keyConfig.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0, k.TotalKeys)
		assert.Equal(t, 1, k.TotalSystems)

		// Force delete item as items have to disconnected first
		// before keyconfig deletion
		_, err = r.Delete(ctx, keyConfig, *repo.NewQuery())
		assert.NoError(t, err)

		_, count, err := m.GetKeyConfigurations(ctx, manager.KeyConfigFilter{})
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestKeyConfigurationsWithGroupID(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should get key configuration and group", func(t *testing.T) {
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		actual, _, err := m.GetKeyConfigurations(ctx, filter)
		assert.NoError(t, err)
		assert.Equal(t, keyConfig.AdminGroupID, actual[0].AdminGroupID)
	})

	t.Run("Should error for non-existing group", func(t *testing.T) {
		badKeyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = uuid.New()
		})

		_, err := m.PostKeyConfigurations(ctx, badKeyConfig)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyAdminGroup)
	})
}

func TestPostKeyConfigurations(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)
	// Create test admin group
	testutils.CreateTestEntities(ctx, t, r, adminGroup)

	t.Run("Should create key configuration", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
			c.AdminGroup = *adminGroup
			c.CreatorID = CreatorID
			c.CreatorName = CreatorName
		})

		ctx := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAdminGroupIAM, keyConfig.AdminGroup.IAMIdentifier},
		)

		actual, err := m.PostKeyConfigurations(ctx, keyConfig)
		assert.NoError(t, err)
		assert.Equal(t, keyConfig.ID, actual.ID)
		assert.Equal(t, keyConfig.Name, actual.Name)
		assert.Equal(t, adminGroup.ID, actual.AdminGroupID)
		assert.Equal(t, CreatorName, actual.CreatorName)
		assert.Equal(t, CreatorID, actual.CreatorID)
	})

	t.Run("Should error when wrong group admin role - TENANT_ADMINISTRATOR", func(t *testing.T) {
		wrongRoleGroup := testutils.NewGroup(func(g *model.Group) {
			g.IAMIdentifier = "KMS_wrong_role_group"
			g.Role = constants.TenantAdminRole
		})

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = wrongRoleGroup.ID
			c.AdminGroup = *wrongRoleGroup
			c.CreatorID = CreatorID
			c.CreatorName = CreatorName
		})

		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{wrongRoleGroup.IAMIdentifier},
		)

		_, err := m.PostKeyConfigurations(ctxWithGroups, keyConfig)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyAdminGroup)
	})

	t.Run("Should error when wrong group admin role - TENANT_AUDITOR", func(t *testing.T) {
		wrongRoleGroup := testutils.NewGroup(func(g *model.Group) {
			g.IAMIdentifier = "KMS_wrong_role_group"
			g.Role = constants.TenantAuditorRole
		})

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = wrongRoleGroup.ID
			c.AdminGroup = *wrongRoleGroup
		})

		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{wrongRoleGroup.IAMIdentifier},
		)

		_, err := m.PostKeyConfigurations(ctxWithGroups, keyConfig)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyAdminGroup)
	})

	t.Run("Should allow creation when user belongs to admin group", func(t *testing.T) {
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAdminGroupIAM, "some_other_group"},
		)

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
			c.AdminGroup = *adminGroup
		})

		actual, err := m.PostKeyConfigurations(ctxWithGroups, keyConfig)
		assert.NoError(t, err)
		assert.Equal(t, keyConfig.ID, actual.ID)
		assert.Equal(t, keyConfig.Name, actual.Name)
	})

	t.Run("Should deny creation when user does not belong to admin group", func(t *testing.T) {
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{"KMS_different_group", "some_other_group"},
		)

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
			c.AdminGroup = *adminGroup
		})

		_, err := m.PostKeyConfigurations(ctxWithGroups, keyConfig)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})

	t.Run("Should deny creation when no groups in context", func(t *testing.T) {
		ctxWithoutGroups := testutils.CreateCtxWithTenant(tenant)

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
			c.AdminGroup = *adminGroup
		})

		_, err := m.PostKeyConfigurations(ctxWithoutGroups, keyConfig)
		assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
	})

	t.Run("Should deny creation when empty groups in context", func(t *testing.T) {
		ctxWithEmptyGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{},
		)

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = adminGroup.ID
		})

		_, err := m.PostKeyConfigurations(ctxWithEmptyGroups, keyConfig)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})

	t.Run("Should error for non-existing admin group", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroupID = uuid.New()
		})

		_, err := m.PostKeyConfigurations(ctx, keyConfig)
		assert.ErrorIs(t, err, manager.ErrInvalidKeyAdminGroup)
	})
}

func TestGetKeyConfigurationsByID(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	r := sql.NewRepository(db)

	expected := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{expected.AdminGroup.IAMIdentifier})

	testutils.CreateTestEntities(ctx, t, r, expected)

	// Create a key configuration with an admin group
	testutils.CreateTestEntities(ctx, t, r, adminGroup, keyConfigWithAdminGroup)

	t.Run("Should get key configuration", func(t *testing.T) {
		actual, err := m.GetKeyConfigurationByID(ctx, expected.ID)
		assert.NoError(t, err)

		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.Name, actual.Name)
	})

	t.Run("Should allow access when system is system user", func(t *testing.T) {
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			constants.SystemUser.String(),
			[]string{},
		)

		actual, err := m.GetKeyConfigurationByID(ctxWithGroups, keyConfigWithAdminGroup.ID)
		assert.NoError(t, err)
		assert.Equal(t, keyConfigWithAdminGroup.ID, actual.ID)
		assert.Equal(t, keyConfigWithAdminGroup.Name, actual.Name)
	})

	t.Run("Should allow access when user belongs to admin group", func(t *testing.T) {
		// Create context with user's IAM groups including the admin group
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAdminGroupIAM, "some_other_group"},
		)

		actual, err := m.GetKeyConfigurationByID(ctxWithGroups, keyConfigWithAdminGroup.ID)
		assert.NoError(t, err)
		assert.Equal(t, keyConfigWithAdminGroup.ID, actual.ID)
		assert.Equal(t, keyConfigWithAdminGroup.Name, actual.Name)
	})

	t.Run("Should deny access when user does not belong to admin group", func(t *testing.T) {
		// Create context with user's IAM groups NOT including the admin group
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{"KMS_different_group", "some_other_group"},
		)

		_, err := m.GetKeyConfigurationByID(ctxWithGroups, keyConfigWithAdminGroup.ID)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})

	t.Run("Should deny access when no groups in context", func(t *testing.T) {
		// Test without any groups in context - should work as before
		ctxWithoutGroups := testutils.CreateCtxWithTenant(tenant)

		_, err := m.GetKeyConfigurationByID(ctxWithoutGroups, expected.ID)
		assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
	})
	t.Run("Should deny access when empty groups in context", func(t *testing.T) {
		// Test with empty groups slice - should work as before
		ctxWithEmptyGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{},
		)

		_, err := m.GetKeyConfigurationByID(ctxWithEmptyGroups, expected.ID)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})
}

func TestUpdateKeyConfigurations(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	r := sql.NewRepository(db)

	id := uuid.New()
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = id
	})

	expected := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.ID = id
	})

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{expected.AdminGroup.IAMIdentifier})

	testutils.CreateTestEntities(ctx, t, r, key, expected, adminGroup, keyConfigWithAdminGroup)

	t.Run("Should update name and description", func(t *testing.T) {
		actual, err := m.UpdateKeyConfigurationByID(
			ctx,
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Name:        ptr.PointTo("test-name"),
				Description: ptr.PointTo("test-description"),
			},
		)
		assert.NoError(t, err)

		expected := expected
		expected.Name = "test-name"
		expected.Description = "test-description"
		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.Name, actual.Name)
	})

	t.Run("Should error on empty name", func(t *testing.T) {
		_, err := m.UpdateKeyConfigurationByID(
			ctx,
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Description: ptr.PointTo("test-description"),
				Name:        ptr.PointTo(""),
			},
		)
		assert.ErrorIs(t, err, manager.ErrNameCannotBeEmpty)
	})

	t.Run("Should error on non existing key config", func(t *testing.T) {
		_, err := m.UpdateKeyConfigurationByID(
			ctx,
			uuid.New(),
			cmkapi.KeyConfigurationPatch{
				Description: ptr.PointTo("test-description"),
				Name:        ptr.PointTo("test-name"),
			},
		)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})

	t.Run("Should error updating key config", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced).WithUpdate()
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		_, err := m.UpdateKeyConfigurationByID(
			ctx,
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Description: ptr.PointTo("test-description"),
				Name:        ptr.PointTo("test-name"),
			},
		)
		assert.ErrorIs(t, err, manager.ErrUpdateKeyConfiguration)
	})

	t.Run("Should allow update when user belongs to admin group", func(t *testing.T) {
		// Create context with proper client data including user's IAM groups
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAdminGroupIAM, "some_other_group"},
		)

		actual, err := m.UpdateKeyConfigurationByID(
			ctxWithGroups,
			keyConfigWithAdminGroup.ID,
			cmkapi.KeyConfigurationPatch{
				Name:        ptr.PointTo("updated-name"),
				Description: ptr.PointTo("updated-description"),
			},
		)
		assert.NoError(t, err)
		assert.Equal(t, keyConfigWithAdminGroup.ID, actual.ID)
		assert.Equal(t, "updated-name", actual.Name)
		assert.Equal(t, "updated-description", actual.Description)
	})

	t.Run("Should deny update when user does not belongs to admin group", func(t *testing.T) {
		// Create context with proper client data with user's IAM groups NOT including the admin group
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{"KMS_different_group", "some_other_group"},
		)

		_, err := m.UpdateKeyConfigurationByID(
			ctxWithGroups,
			keyConfigWithAdminGroup.ID,
			cmkapi.KeyConfigurationPatch{
				Name:        ptr.PointTo("updated-name"),
				Description: ptr.PointTo("updated-description"),
			},
		)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})

	t.Run("Should deny update when no groups in context", func(t *testing.T) {
		ctxWithoutGroups := testutils.CreateCtxWithTenant(tenant)

		_, err := m.UpdateKeyConfigurationByID(
			ctxWithoutGroups,
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Name:        ptr.PointTo("backward-compat-name"),
				Description: ptr.PointTo("backward-compat-description"),
			},
		)
		assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
	})

	t.Run("Should deny update when empty groups in context", func(t *testing.T) {
		// Test with empty groups slice - should work as before
		ctxWithEmptyGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{},
		)

		_, err := m.UpdateKeyConfigurationByID(
			ctxWithEmptyGroups,
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Name:        ptr.PointTo("empty-groups-name"),
				Description: ptr.PointTo("empty-groups-description"),
			},
		)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})
}

func TestDeleteKeyConfiguration(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	r := sql.NewRepository(db)

	expected := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
	})

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{expected.AdminGroup.IAMIdentifier})

	bytes, err := json.Marshal(&[]string{"tag1"})
	assert.NoError(t, err)
	tags := testutils.NewTag(func(t *model.Tag) {
		t.ID = expected.ID
		t.Values = bytes
	})

	testutils.CreateTestEntities(ctx, t, r, expected, tags, adminGroup, keyConfigWithAdminGroup)

	t.Run("Should delete key configuration and it's tags", func(t *testing.T) {
		err := m.DeleteKeyConfigurationByID(ctx, expected.ID)
		assert.NoError(t, err)

		count, err := r.Count(
			ctx,
			&model.KeyConfiguration{},
			*repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(
						repo.IDField, expected.ID),
				),
			),
		)
		assert.Equal(t, 0, count)
		assert.NoError(t, err)

		count, err = r.Count(ctx, &model.Tag{ID: expected.ID}, *repo.NewQuery())
		assert.Equal(t, 0, count)
		assert.NoError(t, err)
	})

	t.Run("Should error on delete key configuration on connected systems", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		sys := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})
		ctx := testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})

		testutils.CreateTestEntities(ctx, t, r, keyConfig, sys)
		err := m.DeleteKeyConfigurationByID(ctx, keyConfig.ID)
		assert.ErrorIs(t, err, manager.ErrDeleteKeyConfiguration)
	})

	t.Run("Should allow access when user belongs to admin group", func(t *testing.T) {
		// Create context with proper client data including user's IAM groups
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{testAdminGroupIAM, "some_other_group"},
		)

		err := m.DeleteKeyConfigurationByID(ctxWithGroups, keyConfigWithAdminGroup.ID)
		assert.NoError(t, err)
	})

	t.Run("Should deny delete when user does not belong to admin group", func(t *testing.T) {
		// Create a key config for this test
		keyConfigForDelete := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfigForDelete)

		// Create context with proper client data with user's IAM groups NOT including the admin group
		ctxWithGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{"KMS_different_group", "some_other_group"},
		)

		err := m.DeleteKeyConfigurationByID(ctxWithGroups, keyConfigForDelete.ID)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})

	t.Run("Should deny delete when empty groups in context", func(t *testing.T) {
		// Create a key config for this test
		keyConfigForDelete := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfigForDelete)

		// Test with empty groups slice - should work as before
		ctxWithEmptyGroups := testutils.InjectClientDataIntoContext(
			ctx,
			"example-user",
			[]string{},
		)

		err := m.DeleteKeyConfigurationByID(ctxWithEmptyGroups, keyConfigForDelete.ID)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotAllowed)
	})
}

func TestTenantConfigManager_GetCertificates(t *testing.T) {
	privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	assert.NoError(t, err)

	m, db, tenant := SetupKeyConfigManager(t)

	t.Run("Should get certificates", func(t *testing.T) {
		cfg := setupCfg(t)
		ctlg, err := catalog.New(t.Context(), &cfg)
		assert.NoError(t, err)
		certManager := manager.NewCertificateManager(
			t.Context(),
			sql.NewRepository(db),
			ctlg,
			&cfg.Certificates,
		)

		ctx := testutils.CreateCtxWithTenant(tenant)

		certManager.SetClient(CertificateIssuerMock{NewCertificateChain: func() string {
			return testutils.CreateCertificateChain(t, pkix.Name{
				Country:            []string{"DE"},
				Organization:       []string{"KCM"},
				OrganizationalUnit: []string{"OpenKCM", "account", "landscape"},
				Locality:           []string{"LOCAL/CMK"},
				CommonName:         "MyCert",
			}, privateKey)
		}})

		_, privateKey, err = certManager.RequestNewCertificate(ctx, privateKey,
			model.RequestCertArgs{
				CertPurpose: model.CertificatePurposeTenantDefault,
				Supersedes:  nil,
				CommonName:  "MyCert",
				Locality:    []string{"LOCAL/CMK"},
			})
		assert.NoError(t, err)

		tenantSubject := "CN=MyCert,OU=OpenKCM/account/landscape,O=KCM,L=LOCAL/CMK,C=DE"

		certs, err := m.GetClientCertificates(ctx)

		assert.NoError(t, err)
		assert.Len(t, certs[model.CertificatePurposeTenantDefault], 1)
		assert.Len(t, certs[model.CertificatePurposeCrypto], 1)
		assert.Equal(t, tenantSubject,
			certs[model.CertificatePurposeTenantDefault][0].Subject)
		assert.Equal(t, TestCertURL,
			certs[model.CertificatePurposeTenantDefault][0].RootCA)
		assert.Equal(t, cryptoSubject,
			certs[model.CertificatePurposeCrypto][0].Subject)
		assert.Equal(t, TestCertURL,
			certs[model.CertificatePurposeCrypto][0].RootCA)
	})
}
