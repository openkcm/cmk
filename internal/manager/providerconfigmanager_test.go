package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/multitenancy"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func SetupProviderManager(t *testing.T) (*manager.ProviderConfigManager, string, *multitenancy.DB) {
	t.Helper()

	svcRegistry := testutils.NewTestPlugins()
	cfg := &config.Config{}

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	m := manager.NewProviderConfigManager(
		svcRegistry,
		make(map[manager.ProviderCachedKey]*manager.ProviderConfig),
		manager.NewTenantConfigManager(r, svcRegistry, cfg, manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg), nil),
		manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg),
		manager.NewPool(r),
		r,
	)
	return m, tenants[0], db
}

func TestGetPluginAlgorithm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "AES256 Algorithm",
			input:    "AES256",
			expected: "KEY_ALGORITHM_AES256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := manager.GetPluginAlgorithm(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateKeystore(t *testing.T) {
	m, _, _ := SetupProviderManager(t)
	provider, resp, err := m.CreateKeystore(t.Context(), &keystoremanagement.CreateKeystoreRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, providerTest, provider)
	assert.Equal(t, keystoremanagement.CreationStatusActive, resp.Status)

	roleManagementCfg, ok := resp.ToKeystoreConfig().Values["roleManagementConfig"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "test-uuid", roleManagementCfg["localityID"])
	assert.Equal(t, "default.kms.test", roleManagementCfg["commonName"])
}

func TestFillKeystorePool(t *testing.T) {
	m, tenant, db := SetupProviderManager(t)
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	size := 2

	err := m.FillKeystorePool(t.Context(), size)
	assert.NoError(t, err)

	// Verify that keystore pool has been filled
	count, err := r.Count(ctx, &model.Keystore{}, *repo.NewQuery())
	assert.NoError(t, err)

	assert.Equal(t, size, count)
}

func TestGetOrInitProvider(t *testing.T) {
	m, tenant, db := SetupProviderManager(t)
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	cert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeHYOKManagement
	})
	testutils.CreateTestEntities(ctx, t, r, cert)
	tests := []struct {
		name   string
		key    *model.Key
		assert func(t *testing.T, provider *manager.ProviderConfig, err error)
	}{
		{
			name: "Valid Provider",
			key: testutils.NewKey(func(k *model.Key) {
				k.KeyType = cmkapi.KeyTypeHYOK
				k.Provider = providerTest
			}),
			assert: func(t *testing.T, provider *manager.ProviderConfig, err error) {
				t.Helper()

				assert.NoError(t, err)
				assert.NotNil(t, provider)
			},
		},
		{
			name: "Invalid Provider",
			key: testutils.NewKey(func(k *model.Key) {
				k.KeyType = cmkapi.KeyTypeHYOK
				k.Provider = "GCP"
			}),
			assert: func(t *testing.T, provider *manager.ProviderConfig, err error) {
				t.Helper()

				assert.Error(t, err)
				assert.Nil(t, provider)
				assert.ErrorIs(t, err, manager.ErrPluginNotFound)
				assert.EqualError(t, err, "plugin not found: GCP")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
			provider, err := m.GetOrInitProvider(ctx, tt.key)
			tt.assert(t, provider, err)
		})
	}
}

func TestGetOrInitProvider_ExpiredEntryIsReinitialized(t *testing.T) {
	svcRegistry := testutils.NewTestPlugins()
	cfg := &config.Config{}

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	cert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeHYOKManagement
	})
	testutils.CreateTestEntities(ctx, t, r, cert)

	expiredAt := time.Now().Add(-time.Second)
	expiredCfg := manager.NewProviderConfig(nil, nil, &expiredAt)

	compositeKey := manager.ProviderCachedKey{
		KeyStore: constants.HYOKKeyStore,
		Provider: providerTest,
		Tenant:   tenant,
	}

	m := manager.NewProviderConfigManager(
		svcRegistry,
		map[manager.ProviderCachedKey]*manager.ProviderConfig{
			compositeKey: expiredCfg,
		},
		manager.NewTenantConfigManager(r, svcRegistry, cfg, manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg), nil),
		manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg),
		manager.NewPool(r),
		r,
	)

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyType = cmkapi.KeyTypeHYOK
		k.Provider = providerTest
	})

	provider, err := m.GetOrInitProvider(ctx, key)

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.False(t, provider.IsExpired())
	assert.True(t, provider.Expiration.After(expiredAt))
}

type scriptedKeystoreManagement struct {
	*testplugins.TestKeystoreManagement

	createStatus       keystoremanagement.CreationStatus
	accountID          string
	omitAccountID      bool
	createErr          error
	getStatus          keystoremanagement.CreationStatus
	getErr             error
	finalizeStatus     keystoremanagement.CreationStatus
	finalizeErr        error
	finalizeErrMessage string

	createCalls   int
	getCalls      int
	finalizeCalls int
	seenAccountID string
	seenValues    map[string]any
}

func (s *scriptedKeystoreManagement) CreateKeystore(
	ctx context.Context,
	req *keystoremanagement.CreateKeystoreRequest,
) (*keystoremanagement.CreateKeystoreResponse, error) {
	s.createCalls++
	if s.createErr != nil {
		return nil, s.createErr
	}

	resp, err := s.TestKeystoreManagement.CreateKeystore(ctx, req)
	if err != nil {
		return nil, err
	}

	if s.createStatus != "" {
		resp.Status = s.createStatus
		if !s.omitAccountID {
			resp.AccountID = s.accountID
			if resp.AccountID == "" {
				resp.AccountID = "acc-" + uuid.NewString()
			}
		}
	}

	return resp, nil
}

func (s *scriptedKeystoreManagement) GetKeystoreStatus(
	_ context.Context,
	req *keystoremanagement.GetKeystoreStatusRequest,
) (*keystoremanagement.GetKeystoreStatusResponse, error) {
	s.getCalls++
	s.seenAccountID = req.AccountID
	if s.getErr != nil {
		return nil, s.getErr
	}

	return &keystoremanagement.GetKeystoreStatusResponse{Status: s.getStatus}, nil
}

func (s *scriptedKeystoreManagement) FinalizeKeystoreSetup(
	_ context.Context,
	req *keystoremanagement.FinalizeKeystoreSetupRequest,
) (*keystoremanagement.FinalizeKeystoreSetupResponse, error) {
	s.finalizeCalls++
	s.seenAccountID = req.AccountID
	s.seenValues = req.Values
	if s.finalizeErr != nil {
		return nil, s.finalizeErr
	}

	return &keystoremanagement.FinalizeKeystoreSetupResponse{
		Status:       s.finalizeStatus,
		ErrorMessage: s.finalizeErrMessage,
		RoleManagementConfig: keystoremanagement.ManagementConfig{
			LocalityID: "finalized-locality",
			CommonName: "finalized.kms.test",
			AccessData: common.KeystoreConfig{Values: map[string]any{"accountId": req.AccountID}},
		},
	}, nil
}

func setupScriptedProviderManager(
	t *testing.T,
	scripted *scriptedKeystoreManagement,
) (*manager.ProviderConfigManager, repo.Repo) {
	t.Helper()

	svcRegistry := testutils.NewTestPlugins(
		testplugins.WithKeystoreManagement(testplugins.Name, scripted),
	)
	cfg := &config.Config{}
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})
	r := sql.NewRepository(db)
	m := manager.NewProviderConfigManager(
		svcRegistry,
		make(map[manager.ProviderCachedKey]*manager.ProviderConfig),
		manager.NewTenantConfigManager(r, svcRegistry, cfg, manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg), nil),
		manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg),
		manager.NewPool(r),
		r,
	)

	return m, r
}

func listKeystores(t *testing.T, r repo.Repo) []*model.Keystore {
	t.Helper()

	var keystores []*model.Keystore
	err := r.List(t.Context(), &model.Keystore{}, &keystores, *repo.NewQuery())
	require.NoError(t, err)

	return keystores
}

func TestFillKeystorePool_KeepsPendingUntilActivation(t *testing.T) {
	scripted := &scriptedKeystoreManagement{
		TestKeystoreManagement: testplugins.NewTestKeystoreManagement(),
		createStatus:           keystoremanagement.CreationStatusPendingActivation,
		accountID:              "acc-pending",
		getStatus:              keystoremanagement.CreationStatusPendingActivation,
	}
	m, r := setupScriptedProviderManager(t, scripted)

	err := m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)

	keystores := listKeystores(t, r)
	require.Len(t, keystores, 1)
	assert.Equal(t, model.KeystoreStatusPendingActivation, keystores[0].Status)

	var stored map[string]any
	require.NoError(t, json.Unmarshal(keystores[0].Config, &stored))
	assert.Equal(t, "acc-pending", stored["accountId"])
	values, ok := stored["values"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, values)

	err = m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, scripted.createCalls)
	assert.Equal(t, 1, scripted.getCalls)
	assert.Empty(t, scripted.finalizeCalls)
	assert.Equal(t, "acc-pending", scripted.seenAccountID)
	assert.Len(t, listKeystores(t, r), 1)
}

func TestFillKeystorePool_FinalizesPendingBeforeCreatingMore(t *testing.T) {
	scripted := &scriptedKeystoreManagement{
		TestKeystoreManagement: testplugins.NewTestKeystoreManagement(),
		getStatus:              keystoremanagement.CreationStatusActive,
		finalizeStatus:         keystoremanagement.CreationStatusActive,
	}
	m, r := setupScriptedProviderManager(t, scripted)
	pool := manager.NewPool(r)

	raw, err := json.Marshal(map[string]any{
		"accountId": "acc-ready",
		"values":    map[string]any{"landscape": "dev"},
	})
	require.NoError(t, err)
	_, err = pool.Add(t.Context(), &model.Keystore{
		ID:       uuid.New(),
		Provider: testplugins.Name,
		Config:   raw,
		Status:   model.KeystoreStatusPendingActivation,
	})
	require.NoError(t, err)

	err = m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)

	assert.Equal(t, 0, scripted.createCalls)
	assert.Equal(t, 1, scripted.finalizeCalls)
	assert.Equal(t, "acc-ready", scripted.seenAccountID)
	assert.Equal(t, "dev", scripted.seenValues["landscape"])

	keystores := listKeystores(t, r)
	require.Len(t, keystores, 1)
	assert.Equal(t, model.KeystoreStatusActive, keystores[0].Status)

	var stored map[string]any
	require.NoError(t, json.Unmarshal(keystores[0].Config, &stored))
	roleCfg, ok := stored["roleManagementConfig"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "finalized-locality", roleCfg["localityID"])
}

func TestFillKeystorePool_PollFailureLeavesPending(t *testing.T) {
	scripted := &scriptedKeystoreManagement{
		TestKeystoreManagement: testplugins.NewTestKeystoreManagement(),
		createStatus:           keystoremanagement.CreationStatusPendingActivation,
		accountID:              "acc-flaky",
		getErr:                 errors.New("temporary outage"),
	}
	m, r := setupScriptedProviderManager(t, scripted)

	err := m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)

	err = m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)

	assert.Equal(t, 1, scripted.createCalls)
	assert.Equal(t, 1, scripted.getCalls)
	keystores := listKeystores(t, r)
	require.Len(t, keystores, 1)
	assert.Equal(t, model.KeystoreStatusPendingActivation, keystores[0].Status)
}

func TestFillKeystorePool_FailedStatusDropsOutOfPool(t *testing.T) {
	scripted := &scriptedKeystoreManagement{
		TestKeystoreManagement: testplugins.NewTestKeystoreManagement(),
		createStatus:           keystoremanagement.CreationStatusPendingActivation,
		accountID:              "acc-failed",
		getStatus:              keystoremanagement.CreationStatusFailed,
	}
	m, r := setupScriptedProviderManager(t, scripted)

	err := m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)

	err = m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)

	assert.Equal(t, 2, scripted.createCalls)
	keystores := listKeystores(t, r)
	require.Len(t, keystores, 2)

	statuses := map[string]int{}
	for _, ks := range keystores {
		statuses[ks.Status]++
	}
	assert.Equal(t, 1, statuses[model.KeystoreStatusFailed])
	assert.Equal(t, 1, statuses[model.KeystoreStatusPendingActivation])
}

func TestFillKeystorePool_CreateFailedWithoutAccountIsNotStored(t *testing.T) {
	scripted := &scriptedKeystoreManagement{
		TestKeystoreManagement: testplugins.NewTestKeystoreManagement(),
		createStatus:           keystoremanagement.CreationStatusFailed,
		omitAccountID:          true,
	}
	m, r := setupScriptedProviderManager(t, scripted)

	err := m.FillKeystorePool(t.Context(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, scripted.createCalls)
	assert.Empty(t, listKeystores(t, r))
}
