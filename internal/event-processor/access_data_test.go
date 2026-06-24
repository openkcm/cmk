package eventprocessor_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// accessDataTestInstance holds everything needed to exercise CryptoAccessDataSyncer.
type accessDataTestInstance struct {
	syncer   *eventprocessor.CryptoAccessDataSyncer
	repo     repo.Repo
	tenantID string
}

func setupAccessDataTestInstance(
	t *testing.T,
	certsCfg []config.CryptoCert,
	opts ...testplugins.RegistryOption,
) accessDataTestInstance {
	t.Helper()

	var certsSource string
	if len(certsCfg) > 0 {
		b, err := yaml.Marshal(certsCfg)
		require.NoError(t, err)
		certsSource = string(b)
	}

	cfg := &config.Config{}
	if certsSource != "" {
		cfg.CryptoLayer = config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  certsSource,
			},
		}
	}

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})
	cfg.Database = dbCfg

	r := sql.NewRepository(db)
	svcRegistry := testutils.NewTestPlugins(opts...)
	syncer := eventprocessor.NewCryptoAccessDataSyncer(cfg, r, svcRegistry)

	return accessDataTestInstance{syncer: syncer, repo: r, tenantID: tenants[0]}
}

// storeKeystoreConfig stores a KeystoreConfig into TenantConfig via the repo.
func storeKeystoreConfig(t *testing.T, r repo.Repo, ctx context.Context, ksConfig model.KeystoreConfig) {
	t.Helper()
	b, err := json.Marshal(ksConfig)
	require.NoError(t, err)
	require.NoError(t, r.Set(ctx, &model.TenantConfig{Key: "DEFAULT_KEYSTORE", Value: b}))
}

// storeRoleManagementCert stores a role management certificate via the repo.
func storeRoleManagementCert(t *testing.T, r repo.Repo, ctx context.Context) {
	t.Helper()
	cert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeRoleManagement
	})
	require.NoError(t, r.Create(ctx, cert))
}

func TestCryptoAccessDataSyncer_NoCertsConfigured(t *testing.T) {
	inst := setupAccessDataTestInstance(t, nil)
	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)
	result, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestCryptoAccessDataSyncer_NoTenantInContext(t *testing.T) {
	inst := setupAccessDataTestInstance(t, []config.CryptoCert{
		{Name: "cert1", Subject: config.CryptoCertSubject{CommonNamePrefix: "test"}},
	})
	_, err := inst.syncer.SyncAndGetCryptoAccessData(context.Background())

	assert.Error(t, err)
}

func TestCryptoAccessDataSyncer_InvalidYAMLCertSource(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})
	cfg := &config.Config{
		Database: dbCfg,
		CryptoLayer: config.CryptoLayer{
			CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "[not: valid: yaml",
			},
		},
	}
	r := sql.NewRepository(db)
	svcRegistry := testutils.NewTestPlugins()
	syncer := eventprocessor.NewCryptoAccessDataSyncer(cfg, r, svcRegistry)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])

	_, err := syncer.SyncAndGetCryptoAccessData(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load crypto certificates")
}

func TestCryptoAccessDataSyncer_DefaultKeystoreConfigNotFound(t *testing.T) {
	// Cert configured but no DEFAULT_KEYSTORE exists
	inst := setupAccessDataTestInstance(
		t,
		[]config.CryptoCert{
			{Name: "cert1", Subject: config.CryptoCertSubject{CommonNamePrefix: "test", Organization: []string{"org"}}},
		},
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, true)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)

	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)
	_, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get default keystore config")
}

func TestCryptoAccessDataSyncer_CertAlreadyUpToDate(t *testing.T) {
	certName := "cert-up-to-date"
	cert := config.CryptoCert{
		Name: certName,
		Subject: config.CryptoCertSubject{
			CommonNamePrefix:   "test",
			OrganizationalUnit: []string{"xyz"},
		},
	}

	inst := setupAccessDataTestInstance(
		t,
		[]config.CryptoCert{cert},
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, true)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)

	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)

	clientCert := model.NewClientCertificate(cert, inst.tenantID)
	expectedSubject := clientCert.Subject.String()

	// Pre-populate keystore config with cert already trusted at the expected subject.
	storeKeystoreConfig(t, inst.repo, ctx, model.KeystoreConfig{
		CryptoAccessData: map[string]model.CryptoConfig{
			certName: {
				Subject:    expectedSubject,
				AccessData: model.KeystoreAccessData{"existingKey": "existingVal"},
			},
		},
	})

	result, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.NoError(t, err)
	require.Contains(t, result, certName)
	assert.Equal(t, expectedSubject, result[certName][model.CertificateSubjectKey])
	assert.Equal(t, "existingVal", result[certName]["existingKey"])
}

func TestCryptoAccessDataSyncer_NewCertGrantsTrust(t *testing.T) {
	certName := "cert-new"
	cert := config.CryptoCert{
		Name:    certName,
		Subject: config.CryptoCertSubject{CommonNamePrefix: "new", OrganizationalUnit: []string{"xyz"}},
	}

	inst := setupAccessDataTestInstance(
		t,
		[]config.CryptoCert{cert},
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, true)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)

	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)

	// Keystore config exists but has no entry for this cert.
	storeKeystoreConfig(t, inst.repo, ctx, model.KeystoreConfig{})
	storeRoleManagementCert(t, inst.repo, ctx)

	result, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.NoError(t, err)
	require.Contains(t, result, certName)
	assert.Contains(t, result[certName], model.CertificateSubjectKey)
}

func TestCryptoAccessDataSyncer_SubjectChangedRegranted(t *testing.T) {
	certName := "cert-subject-change"
	cert := config.CryptoCert{
		Name:    certName,
		Subject: config.CryptoCertSubject{CommonNamePrefix: "test", OrganizationalUnit: []string{"abc"}},
	}

	inst := setupAccessDataTestInstance(
		t,
		[]config.CryptoCert{cert},
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, true)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)

	// Pre-populate with a different (old) subject.
	storeKeystoreConfig(t, inst.repo, ctx, model.KeystoreConfig{
		CryptoAccessData: map[string]model.CryptoConfig{
			certName: {
				Subject:    "/OU=abc/CN=test",
				AccessData: model.KeystoreAccessData{"oldKey": "oldVal"},
			},
		},
	})
	storeRoleManagementCert(t, inst.repo, ctx)

	result, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.NoError(t, err)
	require.Contains(t, result, certName)

	// Subject should be the new one, not the old one.
	newSubject := result[certName][model.CertificateSubjectKey]
	assert.NotEqual(t, "/OU=abc/CN=test", newSubject)
}

func TestCryptoAccessDataSyncer_NoRoleManagementCert(t *testing.T) {
	certName := "cert1"
	cert := config.CryptoCert{
		Name:    certName,
		Subject: config.CryptoCertSubject{CommonNamePrefix: "test", Organization: []string{"org"}},
	}

	inst := setupAccessDataTestInstance(
		t,
		[]config.CryptoCert{cert},
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, true)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)

	// Keystore config exists but cert is not trusted → will try to grant trust → needs role cert.
	storeKeystoreConfig(t, inst.repo, ctx, model.KeystoreConfig{})
	// No role management cert stored.

	_, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get role management certificate")
}

func TestCryptoAccessDataSyncer_NoDefaultKeystorePlugin(t *testing.T) {
	certName := "cert1"
	cert := config.CryptoCert{
		Name:    certName,
		Subject: config.CryptoCertSubject{CommonNamePrefix: "test", OrganizationalUnit: []string{"xyz"}},
	}

	// Key management plugin is NOT marked as default (isDefault=false).
	inst := setupAccessDataTestInstance(
		t,
		[]config.CryptoCert{cert},
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, false)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)

	storeKeystoreConfig(t, inst.repo, ctx, model.KeystoreConfig{})

	_, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no keystore management plugin available")
}

func TestCryptoAccessDataSyncer_MultipleCerts(t *testing.T) {
	cert1Name := "cert-alpha"
	cert2Name := "cert-beta"

	certs := []config.CryptoCert{
		{
			Name:    cert1Name,
			Subject: config.CryptoCertSubject{CommonNamePrefix: "alpha", OrganizationalUnit: []string{"alpha-org"}},
		},
		{
			Name:    cert2Name,
			Subject: config.CryptoCertSubject{CommonNamePrefix: "beta", OrganizationalUnit: []string{"beta-org"}},
		},
	}

	inst := setupAccessDataTestInstance(
		t,
		certs,
		testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, true)),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	ctx := cmkcontext.CreateTenantContext(t.Context(), inst.tenantID)

	// cert1 already trusted; cert2 is new.
	clientCert1 := model.NewClientCertificate(certs[0], inst.tenantID)
	cert1Subject := clientCert1.Subject.String()

	storeKeystoreConfig(t, inst.repo, ctx, model.KeystoreConfig{
		CryptoAccessData: map[string]model.CryptoConfig{
			cert1Name: {
				Subject:    cert1Subject,
				AccessData: model.KeystoreAccessData{"region": "eu-central-1"},
			},
		},
	})
	storeRoleManagementCert(t, inst.repo, ctx)

	result, err := inst.syncer.SyncAndGetCryptoAccessData(ctx)

	assert.NoError(t, err)
	assert.Contains(t, result, cert1Name)
	assert.Contains(t, result, cert2Name)
	assert.Equal(t, cert1Subject, result[cert1Name][model.CertificateSubjectKey])
	assert.Contains(t, result[cert2Name], model.CertificateSubjectKey)
}
