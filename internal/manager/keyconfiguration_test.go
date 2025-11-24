package manager_test

import (
	"crypto/x509/pkix"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
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
	TestCertURL   = "https://aia.pki.co.test.com/aia/TEST%20Cloud%20Root%20CA.crt"
	cryptoSubject = "CryptoCert"
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

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.KeyConfiguration{},
			&model.Key{},
			&model.KeyLabel{},
			&model.Certificate{},
			&model.System{},
			&model.Group{},
			&model.TenantConfig{},
		},
	})

	cfg := setupCfg(t)
	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	dbRepository := sql.NewRepository(db)
	certManager := manager.NewCertificateManager(t.Context(), dbRepository, ctlg, &cfg.Certificates)
	m := manager.NewKeyConfigManager(dbRepository, certManager, &cfg)

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
		testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {}),
		testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {}),
	}

	for _, i := range expected {
		testutils.CreateTestEntities(ctx, t, r, i)
	}

	t.Run("Should get key configuration", func(t *testing.T) {
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		actual, total, err := m.GetKeyConfigurations(testutils.CreateCtxWithTenant(tenant), filter)
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

	t.Run("Should err getting key configuration", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		_, _, err := m.GetKeyConfigurations(t.Context(), filter)
		assert.ErrorIs(t, err, manager.ErrQueryKeyConfigurationList)
	})
}

func TestTotalSystemAndKey(t *testing.T) {
	t.Run("Should get keyconfig with two keys and one system", func(t *testing.T) {
		m, db, tenant := SetupKeyConfigManager(t)
		assert.NotNil(t, m)
		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
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
		k, err := m.GetKeyConfigurationByID(testutils.CreateCtxWithTenant(tenant), keyConfig.ID)
		assert.NoError(t, err)
		assert.Equal(t, 2, k.TotalKeys)
		assert.Equal(t, 1, k.TotalSystems)
	})

	t.Run("Should get no entries on deleted keyconfig with items referencing it", func(t *testing.T) {
		m, db, tenant := SetupKeyConfigManager(t)
		assert.NotNil(t, m)
		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
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
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	expectedKeyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	testutils.CreateTestEntities(ctx, t, r, expectedKeyConfig)

	t.Run("Should get key configuration and group", func(t *testing.T) {
		filter := manager.KeyConfigFilter{Skip: constants.DefaultSkip, Top: constants.DefaultTop}
		actual, _, err := m.GetKeyConfigurations(ctx, filter)
		assert.NoError(t, err)
		assert.Equal(t, expectedKeyConfig.AdminGroupID, actual[0].AdminGroupID)
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

func TestGetKeyConfigurationsByID(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	expected := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	testutils.CreateTestEntities(ctx, t, r, expected)

	t.Run("Should get key configuration", func(t *testing.T) {
		actual, err := m.GetKeyConfigurationByID(testutils.CreateCtxWithTenant(tenant), expected.ID)
		assert.NoError(t, err)

		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.Name, actual.Name)
	})

	t.Run("Should err getting key configuration", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		_, err := m.GetKeyConfigurationByID(testutils.CreateCtxWithTenant(tenant), uuid.New())
		assert.ErrorIs(t, err, manager.ErrGettingKeyConfigByID)
	})
}

func TestUpdateKeyConfigurations(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	id := uuid.New()
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = id
	})

	expected := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.ID = id
		c.Tags = []model.KeyConfigurationTag{
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag1",
				},
			},
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag2",
				},
			},
		}
	})

	testutils.CreateTestEntities(ctx, t, r, key, expected)

	t.Run("Should update name and description", func(t *testing.T) {
		actual, err := m.UpdateKeyConfigurationByID(
			testutils.CreateCtxWithTenant(tenant),
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

	t.Run("Should keep tags on key config update", func(t *testing.T) {
		_, err := m.UpdateKeyConfigurationByID(
			testutils.CreateCtxWithTenant(tenant),
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Name:        ptr.PointTo("test-name"),
				Description: ptr.PointTo("test-description"),
			},
		)
		assert.NoError(t, err)

		actual, err := m.GetKeyConfigurationByID(testutils.CreateCtxWithTenant(tenant), expected.ID)
		assert.NoError(t, err)

		assert.Equal(t, expected.Tags, actual.Tags)
	})

	t.Run("Should error on empty name", func(t *testing.T) {
		_, err := m.UpdateKeyConfigurationByID(
			testutils.CreateCtxWithTenant(tenant),
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
			testutils.CreateCtxWithTenant(tenant),
			uuid.New(),
			cmkapi.KeyConfigurationPatch{
				Description: ptr.PointTo("test-description"),
				Name:        ptr.PointTo("test-name"),
			},
		)
		assert.ErrorIs(t, err, manager.ErrGettingKeyConfigByID)
	})

	t.Run("Should error updating key config", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced).WithUpdate()
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		_, err := m.UpdateKeyConfigurationByID(
			testutils.CreateCtxWithTenant(tenant),
			expected.ID,
			cmkapi.KeyConfigurationPatch{
				Description: ptr.PointTo("test-description"),
				Name:        ptr.PointTo("test-name"),
			},
		)
		assert.ErrorIs(t, err, manager.ErrUpdateKeyConfiguration)
	})
}

func TestDeleteKeyConfiguration(t *testing.T) {
	m, db, tenant := SetupKeyConfigManager(t)
	assert.NotNil(t, m)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	expected := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	testutils.CreateTestEntities(ctx, t, r, expected)

	t.Run("Should delete key configuration", func(t *testing.T) {
		err := m.DeleteKeyConfigurationByID(testutils.CreateCtxWithTenant(tenant), expected.ID)
		assert.NoError(t, err)
	})

	t.Run("Should error on delete key configuration on connected systems", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		sys := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})

		testutils.CreateTestEntities(ctx, t, r, keyConfig, sys)
		err := m.DeleteKeyConfigurationByID(ctx, keyConfig.ID)
		assert.ErrorIs(t, err, manager.ErrDeleteKeyConfiguration)
	})

	t.Run("Should error on delete key configuration db error", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced).WithDelete()
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		err := m.DeleteKeyConfigurationByID(testutils.CreateCtxWithTenant(tenant), expected.ID)
		assert.ErrorIs(t, err, manager.ErrDeleteKeyConfiguration)
	})
}

func TestTenantConfigManager_GetCertificates(t *testing.T) {
	privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	assert.NoError(t, err)

	m, db, tenant := SetupKeyConfigManager(t)

	t.Run("Should get certificates", func(t *testing.T) {
		cfg := setupCfg(t)
		ctlg, err := catalog.New(t.Context(), cfg)
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
				Organization:       []string{"SAP SE"},
				OrganizationalUnit: []string{"SAP Cloud Platform Clients", "subAccount", "landscape"},
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

		tenantSubject := "CN=MyCert,OU=landscape+OU=subAccount+OU=SAP Cloud Platform Clients,O=SAP SE,L=LOCAL/CMK,C=DE"

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
