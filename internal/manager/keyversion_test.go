package manager_test

import (
	"context"
	"testing"

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
	userManager := manager.NewUserManager(s.r, cmkAuditor)
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
					kv.Key.ID = keyID
					kv.KeyID = keyID
					kv.NativeID = "version-1"
				}),
				*testutils.NewKeyVersion(func(kv *model.KeyVersion) {
					kv.Key.ID = keyID
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
}
