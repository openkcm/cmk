package authz_policy_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

// TestEventReconciler_AuthzPolicy verifies that the InternalEventReconcilerRole
// policy grants sufficient repo access for the CryptoReconciler
//
// A key and a CONNECTED system sharing the same KeyConfigurationID are seeded so
// that getRegionsByKeyID exercises Key:First and System:Count+List.
func TestEventReconciler_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalEventReconcilerRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	cfg := &config.Config{Database: dbCfg}

	reconciler, err := eventprocessor.NewCryptoReconciler(
		t.Context(), cfg, authzRepo, testutils.NewTestPlugins(), nil,
	)
	assert.NoError(t, err)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	system := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		s.Status = cmkapi.SystemStatusCONNECTED
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, key, system)

	t.Run("InternalEventReconcilerRole allows deriving connected regions for key", func(t *testing.T) {
		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    key.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeKeyEnable.String(), dataBytes)
		handler, err := reconciler.GetHandlerByJobType(eventprocessor.JobTypeKeyEnable.String())
		assert.NoError(t, err)

		_, err = handler.ResolveTasks(ctx, job)
		// Check error is not authz related
		assert.ErrorIs(t, err, eventprocessor.ErrNoConnectedRegionsForKey)
	})

	// TestCryptoAccessDataSyncer_AuthzPolicy verifies that InternalEventReconcilerRole
	// grants the repo access needed by CryptoAccessDataSyncer.SyncAndGetCryptoAccessData:
	//   - TenantConfig: First (getDefaultKeystoreConfig)
	//   - TenantConfig: Delete + Create (setDefaultKeystoreConfig via repo.Set)
	//   - Certificate:  First (getRoleManagementCert)
	//
	// Strategy: seed a DEFAULT_KEYSTORE TenantConfig with a crypto cert entry whose
	// subject already matches what the syncer will compute. This causes SyncAndGetCryptoAccessData
	// to exercise TenantConfig:First (read config) and then return early — no plugin
	// call and no repo.Set needed. Then a second sub-test seeds a role-management cert
	// and a keystore config with no entry, exercising the grant-trust path up to the
	// plugin call, confirming Certificate:First and TenantConfig:Set are permitted.
	t.Run("InternalEventReconcilerRole allows CryptoAccessDataSyncer to read TenantConfig and Certificate", func(t *testing.T) {
		certName := "authz-test-cert"
		certCfg := config.CryptoCert{
			Name:    certName,
			Subject: config.CryptoCertSubject{CommonNamePrefix: "authz", Organization: []string{"test-org"}},
		}
		certsYAML, err := yaml.Marshal([]config.CryptoCert{certCfg})
		require.NoError(t, err)

		syncerCfg := &config.Config{
			Database: dbCfg,
			CryptoLayer: config.CryptoLayer{
				CertX509Trusts: commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  string(certsYAML),
				},
			},
		}

		syncer := eventprocessor.NewCryptoAccessDataSyncer(syncerCfg, authzRepo, testutils.NewTestPlugins())

		// Compute the subject the syncer will derive for this tenant.
		clientCert := model.NewClientCertificate(certCfg, tenant)
		expectedSubject := clientCert.Subject.String()

		// Seed DEFAULT_KEYSTORE via plain repo (bypassing authz) so the authz-guarded
		// First read is what we're testing, not the write.
		ksConfig := model.KeystoreConfig{
			CryptoAccessData: map[string]model.CryptoConfig{
				certName: {
					Subject:    expectedSubject,
					AccessData: model.KeystoreAccessData{"authzKey": "authzVal"},
				},
			},
		}
		ksBytes, err := json.Marshal(ksConfig)
		require.NoError(t, err)
		require.NoError(t, r.Set(ctx, &model.TenantConfig{Key: constants.DefaultKeyStore, Value: ksBytes}))

		// SyncAndGetCryptoAccessData under InternalEventReconcilerRole: exercises
		// TenantConfig:First. Cert is already up-to-date so exits without Set or plugin.
		result, err := syncer.SyncAndGetCryptoAccessData(ctx)

		assert.NoError(t, err)
		require.Contains(t, result, certName)
		assert.Equal(t, expectedSubject, result[certName][model.CertificateSubjectKey])
	})

	t.Run("InternalEventReconcilerRole allows CryptoAccessDataSyncer Certificate:First and TenantConfig:Set", func(t *testing.T) {
		// Use a fresh DB so there's no pre-existing keystore config or cert.
		db2, tenants2, dbCfg2 := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})
		tenant2 := tenants2[0]
		ctx2 := cmkcontext.CreateTenantContext(t.Context(), tenant2)
		ctx2, err = cmkcontext.InjectInternalUserData(ctx2, constants.InternalEventReconcilerRole)
		require.NoError(t, err)

		r2 := sql.NewRepository(db2)
		authzRepoLoader2 := authz_loader.NewRepoAuthzLoader(t.Context(), r2, &config.Config{})
		authzRepo2 := authz_repo.NewAuthzRepo(r2, authzRepoLoader2)

		certName := "authz-grant-cert"
		certCfg := config.CryptoCert{
			Name:    certName,
			Subject: config.CryptoCertSubject{CommonNamePrefix: "grant", Organization: []string{"grant-org"}},
		}
		certsYAML, err := yaml.Marshal([]config.CryptoCert{certCfg})
		require.NoError(t, err)

		syncerCfg := &config.Config{
			Database: dbCfg2,
			CryptoLayer: config.CryptoLayer{
				CertX509Trusts: commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  string(certsYAML),
				},
			},
		}

		syncer := eventprocessor.NewCryptoAccessDataSyncer(syncerCfg, authzRepo2, testutils.NewTestPlugins())

		// Seed DEFAULT_KEYSTORE with no CryptoAccessData (cert not yet trusted).
		emptyKS, err := json.Marshal(model.KeystoreConfig{})
		require.NoError(t, err)
		require.NoError(t, r2.Set(ctx2, &model.TenantConfig{Key: constants.DefaultKeyStore, Value: emptyKS}))

		// Seed a role-management cert via plain repo.
		roleManagementCert := testutils.NewCertificate(func(c *model.Certificate) {
			c.ID = uuid.New()
			c.Purpose = model.CertificatePurposeRoleManagement
		})
		require.NoError(t, r2.Create(ctx2, roleManagementCert))

		// SyncAndGetCryptoAccessData will:
		//   1. TenantConfig:First  — read DEFAULT_KEYSTORE
		//   2. Certificate:First   — getRoleManagementCert (authz-guarded)
		//   3. call GrantTrust on plugin (TestKeystoreManagement returns success)
		//   4. TenantConfig:Set (Delete+Create) — write updated keystore config
		result, err := syncer.SyncAndGetCryptoAccessData(ctx2)

		assert.NoError(t, err)
		assert.Contains(t, result, certName)
	})

	t.Run("InternalEventReconcilerRole allows KeyVersion:First", func(t *testing.T) {
		kv := testutils.NewKeyVersion(func(k *model.KeyVersion) {
			k.KeyID = key.ID
		})
		require.NoError(t, r.Create(ctx, kv))

		var got model.KeyVersion
		ck := repo.NewCompositeKey().Where(
			fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), key.ID.String(),
		)
		query := repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)).
			Order(repo.OrderField{Field: repo.RotatedField, Direction: repo.Desc})
		_, err = authzRepo.First(ctx, &got, *query)
		assert.NoError(t, err)
	})

	t.Run("InternalEventReconcilerRole allows Event:Update and Event:Delete", func(t *testing.T) {
		identifier := "authz-test-event-identifier"
		event := &model.Event{
			Identifier: identifier,
			Type:       "test.event.type",
			Data:       []byte(`{}`),
			Status:     "FAILED",
		}
		require.NoError(t, r.Create(ctx, event))

		// Exercise Event:Update via Patch
		patchEvent := &model.Event{Identifier: identifier, ErrorCode: "TEST_CODE", ErrorMessage: "test error"}
		_, err = r.Patch(ctx, patchEvent, *repo.NewQuery())
		assert.NoError(t, err)

		// Exercise Event:Delete
		_, err = r.Delete(ctx, &model.Event{}, *repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.IdentifierField, identifier)),
		))
		assert.NoError(t, err)
	})
}
