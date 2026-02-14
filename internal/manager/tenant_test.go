package manager_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	cmkplugincatalog "github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var IssuerURL = "http://issuer-url"

func SetupTenantManager(t *testing.T, opts ...testutils.TestDBConfigOpt) (
	*manager.TenantManager,
	repo.Repo, []string,
) {
	t.Helper()

	dbCon, tenants, dbCfg := testutils.NewTestDB(
		t, testutils.TestDBConfig{
			CreateDatabase: true,
			WithOrbital:    true,
		}, opts...,
	)

	cfg := &config.Config{
		Database: dbCfg,
	}
	ctx := t.Context()

	r := sql.NewRepository(dbCon)

	ctlg, err := cmkplugincatalog.New(ctx, cfg)
	assert.NoError(t, err)
	reconciler, err := eventprocessor.NewCryptoReconciler(
		ctx, cfg, r,
		ctlg, nil,
	)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)

	f, err := clients.NewFactory(config.Services{})
	assert.NoError(t, err)

	cm := manager.NewCertificateManager(ctx, r, ctlg, &cfg.Certificates)
	um := testutils.NewUserManager()
	tagManager := manager.NewTagManager(r)
	kcm := manager.NewKeyConfigManager(r, cm, um, tagManager, cmkAuditor, cfg)

	sys := manager.NewSystemManager(
		ctx,
		r,
		f,
		reconciler,
		ctlg,
		cfg,
		kcm,
		um,
	)

	km := manager.NewKeyManager(
		r,
		ctlg,
		manager.NewTenantConfigManager(r, ctlg, nil),
		kcm,
		um,
		cm,
		reconciler,
		cmkAuditor,
	)

	migrator := testutils.NewMigrator()

	m := manager.NewTenantManager(r, sys, km, um, cmkAuditor, migrator)

	return m, r, tenants
}

func TestTenantManager(t *testing.T) {
	nTenants := 10
	m, r, tenants := SetupTenantManager(t, testutils.WithGenerateTenants(nTenants))

	t.Run("Should get tenant info", func(t *testing.T) {
		tenant := tenants[5]
		tenantModel, err := m.GetTenant(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Equal(t, tenant, tenantModel.ID)
	},
	)
	t.Run("Should list tenants", func(t *testing.T) {
		tenantsModel, _, err := m.ListTenantInfo(t.Context(), nil, repo.Pagination{})
		assert.NoError(t, err)

		for i := range nTenants {
			assert.Equal(t, tenants[i], tenantsModel[i].ID)
		}
	},
	)
	t.Run("Should delete tenant", func(t *testing.T) {
		tenant := testutils.NewTenant(
			func(t *model.Tenant) {
				t.SchemaName = "test_delete"
				t.DomainURL = "test_delete@test.test"
			},
		)
		err := m.CreateTenant(t.Context(), tenant)
		assert.NoError(t, err)

		ctx := testutils.CreateCtxWithTenant(tenant.ID)
		err = m.DeleteTenant(ctx)
		assert.NoError(t, err)

		_, err = m.GetTenant(ctx)
		assert.ErrorIs(t, err, repo.ErrTenantNotFound)

		count, err := r.Count(ctx, &model.System{}, *repo.NewQuery())
		assert.ErrorIs(t, err, repo.ErrTenantNotFound)
		assert.Equal(t, 0, count)
	},
	)
	t.Run("Should not error on delete non existing tenant", func(t *testing.T) {
		ctx := testutils.CreateCtxWithTenant(uuid.NewString())
		_, err := m.GetTenant(ctx)
		assert.ErrorIs(t, err, repo.ErrTenantNotFound)

		err = m.DeleteTenant(ctx)
		assert.NoError(t, err)
	},
	)
}

func TestOffboardTenant(t *testing.T) {
	m, r, tenants := SetupTenantManager(t)

	keyConfigID := uuid.New()
	key := testutils.NewKey(
		func(k *model.Key) {
			k.KeyConfigurationID = keyConfigID
		},
	)
	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = ptr.PointTo(key.ID)
			k.ID = keyConfigID
		},
	)

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])
	ctx = testutils.InjectClientDataIntoContext(ctx, uuid.NewString(), []string{keyConfig.AdminGroup.IAMIdentifier})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

	t.Run("Should return success", func(t *testing.T) {
		testutils.CreateTestEntities(
			ctx, t, r, testutils.NewSystem(
				func(s *model.System) {
					s.Status = cmkapi.SystemStatusFAILED
					s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
				},
			),
		)
		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingSuccess, result.Status)
	},
	)

	t.Run("Should return in processing on processing systems", func(t *testing.T) {
		testutils.CreateTestEntities(
			ctx, t, r, testutils.NewSystem(
				func(s *model.System) {
					s.Status = cmkapi.SystemStatusPROCESSING
					s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
				},
			),
		)
		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingProcessing, result.Status)
	},
	)

	t.Run("Should return in processing on systems that havent been processed", func(t *testing.T) {
		system := testutils.NewSystem(
			func(s *model.System) {
				s.Status = cmkapi.SystemStatusCONNECTED
				s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
			},
		)
		testutils.CreateTestEntities(ctx, t, r, system)
		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingProcessing, result.Status)

		_, err = r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusPROCESSING, system.Status)
	},
	)
}

func TestGetTenantByID(t *testing.T) {
	m, _, tenants := SetupTenantManager(t, testutils.WithGenerateTenants(1))
	tenant := tenants[0]

	tests := []struct {
		name     string
		tenantID string
		wantErr  bool
	}{
		{
			name:     "should get tenant by ID",
			tenantID: tenant,
			wantErr:  false,
		},
		{
			name:     "should return error for non-existing tenant ID",
			tenantID: "non-existing-tenant",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			result, err := m.GetTenantByID(ctx, tt.tenantID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.tenantID, result.ID)
		},
		)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name          string
		schema        string
		expectedError error
	}{
		{
			name:   "valid schema",
			schema: "KMS_validschema",
		},
		{
			name:          "schema name too long",
			schema:        "KMS_" + strings.Repeat("a", 60), // 64+ characters
			expectedError: manager.ErrSchemaNameLength,
		},
		{
			name:          "schema name too short",
			schema:        "sc",
			expectedError: manager.ErrInvalidSchema,
		},
		{
			name:          "namespace validation fails forbidden prefix",
			schema:        "pg_invalid",
			expectedError: manager.ErrInvalidSchema,
		},
		{
			name:          "namespace validation fails regex check",
			schema:        "invalid@name",
			expectedError: manager.ErrInvalidSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateSchema(tt.schema)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		},
		)
	}
}
