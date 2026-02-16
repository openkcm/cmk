package async_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
	"github.com/openkcm/cmk/utils/crypto"
)

// There is no PKI mock, so credentials for this must be added below
func getConfig(t *testing.T, schCfg config.Scheduler) *config.Config {
	t.Helper()

	return &config.Config{
		Database: integrationutils.DB,
		Plugins: []plugincatalog.PluginConfig{
			integrationutils.SISPlugin(t),
			integrationutils.PKIPlugin(t),
			integrationutils.KeystorePlugin(t),
			integrationutils.KeystoreProviderPlugin(t),
			integrationutils.IDMangementPlugin(t),
		},
		Scheduler: schCfg,
		Certificates: config.Certificates{
			ValidityDays: 7,
		},
		ContextModels: config.ContextModels{
			System: config.System{
				OptionalProperties: map[string]config.SystemProperty{
					SystemRole:   {},
					SystemRoleID: {},
					SystemName:   {},
				},
			},
		},
		KeystorePool: config.KeystorePool{
			Size: 5,
		},
	}
}

// The tests create a random database since the async app receives
// only the link to the database this would cause
// tests to target the wrong database
func overrideDatabase(t *testing.T, a *async.App, db *multitenancy.DB, cfg *config.Config) {
	t.Helper()

	svcRegistry, err := cmkpluginregistry.New(t.Context(), cfg)
	assert.NoError(t, err)

	tenancyRepo := sql.NewRepository(db)

	val := reflect.ValueOf(a).Elem()

	sis, err := manager.NewSystemInformationManager(tenancyRepo, svcRegistry, &cfg.ContextModels.System)
	assert.NoError(t, err)

	sysCl := val.FieldByName("systemClient")
	sysCl = reflect.NewAt(sysCl.Type(), unsafe.Pointer(sysCl.UnsafeAddr())).Elem()
	sysCl.Set(reflect.ValueOf(sis))

	cm := manager.NewCertificateManager(t.Context(), tenancyRepo, svcRegistry, &cfg.Certificates)

	certCl := val.FieldByName("certificateClient")
	certCl = reflect.NewAt(certCl.Type(), unsafe.Pointer(certCl.UnsafeAddr())).Elem()
	certCl.Set(reflect.ValueOf(cm))

	reconciler, err := eventprocessor.NewCryptoReconciler(t.Context(), cfg, tenancyRepo, svcRegistry, nil)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), cfg)
	tc := manager.NewTenantConfigManager(tenancyRepo, svcRegistry, nil)
	um := manager.NewUserManager(tenancyRepo, cmkAuditor)
	tam := manager.NewTagManager(tenancyRepo)
	kc := manager.NewKeyConfigManager(tenancyRepo, cm, um, tam, nil, cfg)
	km := manager.NewKeyManager(tenancyRepo, svcRegistry, tc, kc, um, cm, reconciler, nil)

	hyokCl := val.FieldByName("hyokClient")
	hyokCl = reflect.NewAt(hyokCl.Type(), unsafe.Pointer(hyokCl.UnsafeAddr())).Elem()
	hyokCl.Set(reflect.ValueOf(km))

	ksPoolCl := val.FieldByName("keystorePoolClient")
	ksPoolCl = reflect.NewAt(ksPoolCl.Type(), unsafe.Pointer(ksPoolCl.UnsafeAddr())).Elem()
	ksPoolCl.Set(reflect.ValueOf(km))

	dbCon := val.FieldByName("dbCon")
	dbCon = reflect.NewAt(dbCon.Type(), unsafe.Pointer(dbCon.UnsafeAddr())).Elem()
	dbCon.Set(reflect.ValueOf(db))
}

func SetupTestContainers(t *testing.T, cfg *config.Config) {
	t.Helper()

	testutils.StartRedis(t, &cfg.Scheduler)
}

func setupDatabase(ctx context.Context, t *testing.T, r repo.Repo, keysEnabled bool) {
	t.Helper()

	cert := createTestCertificate(t)

	if keysEnabled {
		group, keyConfig, key := createTestKeyEntities()
		testutils.CreateTestEntities(ctx, t, r, &key, &group, &keyConfig, &cert)
	} else {
		testutils.CreateTestEntities(ctx, t, r, &cert)
	}
}

func createTestCertificate(t *testing.T) model.Certificate {
	t.Helper()

	privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	assert.NoError(t, err)

	certPEM := testutils.CreateCertificatePEM(t, &x509.CertificateRequest{
		Subject: pkix.Name{
			Country:            []string{"DE"},
			Organization:       []string{"EXAMPLE_O"},
			OrganizationalUnit: []string{"EXAMPLE_OU"},
			Locality:           []string{"LOCAL/CMK"},
			CommonName:         "myCert",
		},
	}, privateKey)

	return model.Certificate{
		ID:             uuid.New(),
		CommonName:     "CommonName",
		CertPEM:        string(certPEM),
		Purpose:        model.CertificatePurposeTenantDefault,
		ExpirationDate: time.Now().AddDate(0, 0, -8),
	}
}

func createTestKeyEntities() (model.Group, model.KeyConfiguration, model.Key) {
	group := model.Group{
		ID:          uuid.New(),
		Name:        "testgroup",
		Description: "This is a test group",
		Role:        "testrole",
	}
	keyConfig := model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         "hyok",
		Description:  "This key configuration is used for HANA key store encryption.",
		AdminGroupID: group.ID,
		CreatorID:    uuid.NewString(),
		CreatorName:  "testuser",
	}
	nativeID := "arn:aws:kms:eu-west-2:fake:key/fake-key-id"
	key := model.Key{
		ID:                   uuid.New(),
		Name:                 "hyok",
		KeyType:              constants.KeyTypeHYOK,
		Description:          "This key is used for HANA key store encryption.",
		Algorithm:            "AES256",
		Provider:             "AWS",
		Region:               "eu-west-2",
		NativeID:             &nativeID,
		KeyConfigurationID:   keyConfig.ID,
		ManagementAccessData: json.RawMessage(`{"roleArn": "test"}`),
		CryptoAccessData:     json.RawMessage(`{}`),
		IsPrimary:            true,
		State:                "DISABLED",
	}

	return group, keyConfig, key
}
