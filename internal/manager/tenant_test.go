package manager_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/clients/registry/mapping"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

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

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	assert.NoError(t, err)

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)

	cm := manager.NewCertificateManager(ctx, r, svcRegistry, cfg)
	um := testutils.NewUserManager()
	tagManager := manager.NewTagManager(r)
	kcm := manager.NewKeyConfigManager(r, cm, um, tagManager, cmkAuditor, cfg)

	mappingService := mapping.NewFakeService()
	_, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			mappingv1.RegisterServiceServer(s, mappingService)
		},
	)

	clientsFactory, err := clients.NewFactory(config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: true,
			Address: grpcClient.Target(),
			SecretRef: &commoncfg.SecretRef{
				Type: commoncfg.InsecureSecretType,
			},
		},
	})
	assert.NoError(t, err)

	sys := manager.NewSystemManager(
		ctx,
		r,
		clientsFactory,
		eventFactory,
		svcRegistry,
		cfg,
		kcm,
		um,
	)

	km := manager.NewKeyManager(
		r,
		svcRegistry,
		manager.NewTenantConfigManager(r, svcRegistry, nil),
		kcm,
		um,
		cm,
		eventFactory,
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
			ctx, t, r,
			testutils.NewSystem(
				func(s *model.System) {
					s.Status = cmkapi.SystemStatusDISCONNECTED
					s.KeyConfigurationID = nil
				},
			),
			testutils.NewKey(
				func(k *model.Key) {
					k.KeyConfigurationID = keyConfig.ID
					k.IsPrimary = true
					k.State = string(cmkapi.KeyStateDETACHED)
				},
			),
		)
		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingSuccess, result.Status)
	})

	t.Run("Should return in processing on processing systems", func(t *testing.T) {
		disconnectAllExistingSystems(t, ctx, r)
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
		assert.Equal(t, manager.OffboardingContinueAndWait, result.Status)
	})

	t.Run("Should return in processing on systems that havent been processed", func(t *testing.T) {
		disconnectAllExistingSystems(t, ctx, r)
		system := testutils.NewSystem(
			func(s *model.System) {
				s.Status = cmkapi.SystemStatusCONNECTED
				s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
			},
		)
		testutils.CreateTestEntities(ctx, t, r, system)
		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingContinueAndWait, result.Status)

		_, err = r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusPROCESSING, system.Status)
	})

	t.Run("Should return in processing on keys that havent been processed", func(t *testing.T) {
		disconnectAllExistingSystems(t, ctx, r)
		key := testutils.NewKey(
			func(k *model.Key) {
				k.KeyConfigurationID = keyConfig.ID
				k.IsPrimary = true
				k.State = string(cmkapi.KeyStateENABLED)
			},
		)
		testutils.CreateTestEntities(ctx, t, r, key)

		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingContinueAndWait, result.Status)

		_, err = r.First(ctx, key, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateDETACHING), key.State)
	})

	t.Run("returns error when unlinking connected systems fails", func(t *testing.T) {
		disconnectAllExistingSystems(t, ctx, r)
		system := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusCONNECTED
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})
		testutils.CreateTestEntities(ctx, t, r, system)

		mockSys := &mockSystemManager{unlinkErr: manager.ErrGettingSystemByID}
		m.SetSystemForTests(mockSys)
		_, err := m.OffboardTenant(ctx)
		assert.Error(t, err)
	})

	t.Run("returns ContinueAndWait when unmapping systems returns retryable error", func(t *testing.T) {
		disconnectAllExistingSystems(t, ctx, r)
		system := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusDISCONNECTED
			s.KeyConfigurationID = nil
		})
		testutils.CreateTestEntities(ctx, t, r, system)

		mockSys := &mockSystemManager{unmapErr: status.Error(codes.Internal, "internal")}
		m.SetSystemForTests(mockSys)

		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingContinueAndWait, result.Status)
	})

	t.Run("returns Failed when unmapping systems returns InvalidArgument", func(t *testing.T) {
		disconnectAllExistingSystems(t, ctx, r)

		mockSys := &mockSystemManager{unmapErr: status.Error(codes.InvalidArgument, "invalid argument")}
		m.SetSystemForTests(mockSys)

		result, err := m.OffboardTenant(ctx)
		assert.NoError(t, err)
		assert.Equal(t, manager.OffboardingFailed, result.Status)
	})
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

func TestUnmapSystemErrorCanContinue(t *testing.T) {
	m := &manager.TenantManager{}
	ctx := t.Context()

	t.Run("err is nil", func(t *testing.T) {
		st := m.UnmapSystemErrorCanContinue(ctx, nil)
		assert.Equal(t, manager.OffboardingGoToNextStep, st)
	})

	t.Run("FailedPrecondition with system not linked", func(t *testing.T) {
		err := status.Error(codes.FailedPrecondition, "system is not linked to the tenant")
		st := m.UnmapSystemErrorCanContinue(ctx, err)
		assert.Equal(t, manager.OffboardingGoToNextStep, st)
	})

	t.Run("NotFound with system not found", func(t *testing.T) {
		err := status.Error(codes.NotFound, "system not found")
		st := m.UnmapSystemErrorCanContinue(ctx, err)
		assert.Equal(t, manager.OffboardingGoToNextStep, st)
	})

	t.Run("InvalidArgument", func(t *testing.T) {
		err := status.Error(codes.InvalidArgument, "invalid argument")
		st := m.UnmapSystemErrorCanContinue(ctx, err)
		assert.Equal(t, manager.OffboardingFailed, st)
	})

	t.Run("other errors", func(t *testing.T) {
		err := status.Error(codes.Internal, "some internal error")
		st := m.UnmapSystemErrorCanContinue(ctx, err)
		assert.Equal(t, manager.OffboardingContinueAndWait, st)
	})

	t.Run("non-status error", func(t *testing.T) {
		st := m.UnmapSystemErrorCanContinue(ctx, manager.ErrNoSystem)
		assert.Equal(t, manager.OffboardingContinueAndWait, st)
	})
}

type mockSystemManager struct {
	unlinkErr error
	unmapErr  error
}

func (s *mockSystemManager) UnmapSystemFromRegistry(context.Context, *model.System) error {
	return s.unmapErr
}

func (s *mockSystemManager) UnlinkSystemAction(context.Context, uuid.UUID, string) error {
	return s.unlinkErr
}

func (s *mockSystemManager) GetAllSystems(context.Context, repo.QueryMapper) ([]*model.System, int, error) {
	panic("not implemented")
}

func (s *mockSystemManager) GetSystemByID(context.Context, uuid.UUID) (*model.System, error) {
	panic("not implemented")
}
func (s *mockSystemManager) RefreshSystemsData(context.Context) bool { return true }

func (s *mockSystemManager) LinkSystemAction(context.Context, uuid.UUID, cmkapi.SystemPatch) (*model.System, error) {
	panic("not implemented")
}

func (s *mockSystemManager) GetRecoveryActions(context.Context, uuid.UUID) (cmkapi.SystemRecoveryAction, error) {
	panic("not implemented")
}

func (s *mockSystemManager) SendRecoveryActions(
	context.Context,
	uuid.UUID,
	cmkapi.SystemRecoveryActionBodyAction,
) error {
	panic("not implemented")
}

func (s *mockSystemManager) GetFilters(context.Context) (cmkapi.SystemFilters, error) {
	panic("not implemented")
}

func disconnectAllExistingSystems(t *testing.T, ctx context.Context, r repo.Repo) {
	t.Helper()

	var systems []*model.System
	err := r.List(ctx, model.System{}, &systems, *repo.NewQuery())
	assert.NoError(t, err)
	for _, system := range systems {
		system.Status = cmkapi.SystemStatusDISCONNECTED
		system.KeyConfigurationID = nil
		_, err = r.Patch(ctx, system, *repo.NewQuery().UpdateAll(true))
		assert.NoError(t, err)
	}
}
