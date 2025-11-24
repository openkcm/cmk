package manager_test

import (
	"context"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	regionpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func SetupSystemManager(t *testing.T, clientsFactory *clients.Factory) (
	*manager.SystemManager,
	*multitenancy.DB,
	string,
) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.System{},
			&model.KeyConfiguration{},
			&model.SystemProperty{},
			&model.Event{},
		},
	})

	cfg := config.Config{
		Plugins: testutils.SetupMockPlugins(testutils.SystemInfo),
		BaseConfig: commoncfg.BaseConfig{
			Audit: commoncfg.Audit{
				Endpoint: "http://localhost:4318/v1/logs",
			},
		},
		Database: dbCfg,
	}

	ctlg, err := catalog.New(t.Context(), cfg)
	require.NoError(t, err)

	dbRepository := sql.NewRepository(db)

	eventProcessor, err := eventprocessor.NewCryptoReconciler(
		t.Context(), &cfg, dbRepository,
		ctlg,
	)
	require.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), &cfg)

	if clientsFactory == nil {
		clientsFactory, err = clients.NewFactory(config.Services{})
		assert.NoError(t, err)
	}

	t.Cleanup(func() {
		assert.NoError(t, clientsFactory.Close())
	})

	systemManager := manager.NewSystemManager(
		t.Context(), dbRepository,
		clientsFactory,
		eventProcessor, ctlg, cmkAuditor,
		&cfg,
	)

	return systemManager, db, tenants[0]
}

func registerSystem(ctx context.Context, t *testing.T, systemService *systems.FakeService, externalID, region,
	sysType string, options ...func(*systemgrpc.RegisterSystemRequest),
) {
	t.Helper()

	req := &systemgrpc.RegisterSystemRequest{
		ExternalId:    externalID,
		L2KeyId:       "key123",
		Region:        region,
		Type:          sysType,
		HasL1KeyClaim: true,
	}

	// Apply any optional modifications
	for _, opt := range options {
		opt(req)
	}

	_, err := systemService.RegisterSystem(ctx, req)
	assert.NoError(t, err)
}

func TestNewSystemManager(t *testing.T) {
	t.Run("Should create system manager", func(t *testing.T) {
		m, _, _ := SetupSystemManager(t, nil)

		assert.NotNil(t, m)
	})
}

func TestGetAllSystems(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	system1 := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = &keyConfig.ID
		s.KeyConfigurationName = &keyConfig.Name
		s.Properties = map[string]string{
			"a": "b",
			"b": "c",
		}
	})
	system2 := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		system1,
		system2,
		keyConfig,
	)

	t.Run("Should get all systems", func(t *testing.T) {
		expected := []*model.System{system1, system2}

		filter := manager.SystemFilter{
			Skip: constants.DefaultSkip,
			Top:  constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.NoError(t, err)
		assert.ElementsMatch(t, expected, systems)
		assert.Equal(t, len(expected), total)
	})

	t.Run("Should fail to get all systems", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		filter := manager.SystemFilter{
			Skip: constants.DefaultSkip,
			Top:  constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.Nil(t, systems)
		assert.Equal(t, 0, total)
		assert.ErrorIs(t, err, manager.ErrQuerySystemList)
	})

	t.Run("Should get all systems filtered by keyConfigID", func(t *testing.T) {
		expected := []*model.System{system1}

		filter := manager.SystemFilter{
			KeyConfigID: keyConfig.ID,
			Skip:        constants.DefaultSkip,
			Top:         constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.NoError(t, err)
		assert.Equal(t, expected, systems)
		assert.Equal(t, len(expected), total)
	})

	t.Run("Should fail to get all systems filtered by keyConfigID", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		filter := manager.SystemFilter{
			KeyConfigID: keyConfig.ID,
			Skip:        constants.DefaultSkip,
			Top:         constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.Nil(t, systems)
		assert.Equal(t, 0, total)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotFound)
	})
}

func TestGetAllSystemsFiltered(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	system1 := testutils.NewSystem(func(s *model.System) {
		s.Region = "Region1"
		s.Type = "Type1"
	})

	expected1 := []*model.System{system1}

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		system1,
	)

	t.Run("Should get all systems filtered by region", func(t *testing.T) {
		filter := manager.SystemFilter{
			Region: "Region1",
			Skip:   constants.DefaultSkip,
			Top:    constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.NoError(t, err)
		assert.Equal(t, expected1, systems)
		assert.Equal(t, 1, total)
	})

	t.Run("Should get all systems filtered by type", func(t *testing.T) {
		filter := manager.SystemFilter{
			Type: "Type1",
			Skip: constants.DefaultSkip,
			Top:  constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.NoError(t, err)
		assert.Equal(t, expected1, systems)
		assert.Equal(t, 1, total)
	})

	t.Run("Should fail to get systems filtered by region", func(t *testing.T) {
		filter := manager.SystemFilter{
			Region: "RegionInvalid",
			Skip:   constants.DefaultSkip,
			Top:    constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.NoError(t, err)
		assert.Nil(t, systems)
		assert.Equal(t, 0, total)
	})

	t.Run("Should fail to get systems filtered by type", func(t *testing.T) {
		filter := manager.SystemFilter{
			Type: "TypeInvalid",
			Skip: constants.DefaultSkip,
			Top:  constants.DefaultTop,
		}
		systems, total, err := m.GetAllSystems(ctx, filter)

		assert.NoError(t, err)
		assert.Nil(t, systems)
		assert.Equal(t, 0, total)
	})
}

func TestGetSystemByID(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	system := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(ctx, t, r, system)

	t.Run("Should get system by id", func(t *testing.T) {
		actualSystem, err := m.GetSystemByID(ctx, system.ID)

		assert.Equal(t, system, actualSystem)
		assert.NoError(t, err)
	})

	t.Run("Should fail on get system by id", func(t *testing.T) {
		actualSystem, err := m.GetSystemByID(ctx, uuid.New())

		assert.Nil(t, actualSystem)
		assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
	})

	t.Run("Should not get keyconfig name", func(t *testing.T) {
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(uuid.New())
		})

		testutils.CreateTestEntities(ctx, t, r, system)
		actualSystem, err := m.GetSystemByID(ctx, system.ID)
		assert.Nil(t, actualSystem.KeyConfigurationName)
		assert.NoError(t, err)
	})
}

func TestGetSystemLinkByID(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	keyConfigID := uuid.New()
	system := &model.System{
		ID:                 uuid.New(),
		Identifier:         uuid.New().String(),
		KeyConfigurationID: &keyConfigID,
	}

	testutils.CreateTestEntities(ctx, t, r, system)

	t.Run("Should get system link by id", func(t *testing.T) {
		actualSystem, err := m.GetSystemLinkByID(ctx, system.ID)

		assert.Equal(t, system.KeyConfigurationID, actualSystem)
		assert.NoError(t, err)
	})

	t.Run("Should fail on getting system by id", func(t *testing.T) {
		link, err := m.GetSystemLinkByID(ctx, uuid.New())

		assert.Nil(t, link)
		assert.ErrorIs(t, err, manager.ErrGettingSystemLinkByID)
	})

	t.Run("Should fail getting system by id with empty keyconfig", func(t *testing.T) {
		system := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.New().String(),
			KeyConfigurationID: nil,
		}

		testutils.CreateTestEntities(ctx, t, r, system)
		link, err := m.GetSystemLinkByID(ctx, system.ID)

		assert.Nil(t, link)
		assert.ErrorIs(t, err, manager.ErrKeyConfigurationIDNotFound)
	})
}

func TestEventRetry(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	primaryKeyID := uuid.New()
	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = &primaryKeyID
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should error on retry with system status not failed", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		testutils.CreateTestEntities(ctx, t, r, system)

		_, err := m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
		})
		assert.NoError(t, err)

		_, err = m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
			Retry: ptr.PointTo(true),
		})
		assert.ErrorIs(t, err, manager.ErrRetryNonFailedSystem)
	})

	t.Run("Should error on retry without previous event", func(t *testing.T) {
		system := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})
		testutils.CreateTestEntities(ctx, t, r, system)

		_, err := m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
			Retry:              ptr.PointTo(true),
		})
		assert.ErrorIs(t, err, eventprocessor.ErrNoPreviousEvent)
	})

	sucessRetry := func(t *testing.T, ctx context.Context, system *model.System) {
		t.Helper()

		_, err := r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)

		// System set to processing on retry
		assert.Equal(t, cmkapi.SystemStatusPROCESSING, system.Status)
	}

	tests := []struct {
		name      string
		eventType proto.TaskType
		f         func(t *testing.T, ctx context.Context, system *model.System)
	}{
		{
			name:      "should retry link event",
			eventType: proto.TaskType_SYSTEM_LINK,
			f:         sucessRetry,
		},
		{
			name:      "should retry switch event",
			eventType: proto.TaskType_SYSTEM_SWITCH,
			f:         sucessRetry,
		},
		{
			name:      "should retry unlink event",
			eventType: proto.TaskType_SYSTEM_UNLINK,
			f:         sucessRetry,
		},
		{
			name:      "should fail on second retry",
			eventType: proto.TaskType_SYSTEM_UNLINK,
			f: func(t *testing.T, ctx context.Context, system *model.System) {
				t.Helper()

				_, err := m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
					Retry:              ptr.PointTo(true),
				})
				assert.ErrorIs(t, err, manager.ErrRetryNonFailedSystem)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := testutils.NewSystem(func(s *model.System) {
				s.Status = cmkapi.SystemStatusFAILED
			})
			testutils.CreateTestEntities(ctx, t, r, system)

			// Represent task failure
			err := r.Create(ctx, &model.Event{
				Identifier: system.ID.String(),
				Type:       tt.eventType.String(),
				Data:       []byte("{}"),
			})
			assert.NoError(t, err)
			err = db.WithTenant(ctx, "orbital", func(tx *multitenancy.DB) error {
				job := orbital.Job{
					ID:         uuid.New(),
					ExternalID: system.ID.String(),
					Data:       []byte("{}"),
					Type:       tt.eventType.String(),
					Status:     orbital.JobStatusFailed,
				}

				return tx.Table("jobs").Create(&job).Error
			})
			assert.NoError(t, err)

			_, err = m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
				Retry:              ptr.PointTo(true),
			})
			assert.NoError(t, err)

			tt.f(t, ctx, system)
		})
	}
}

func TestEventSelector(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	t.Run("Should link on new keyconfig without pkey", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		updatedSystem := testutils.NewSystem(func(_ *model.System) {})

		event, err := m.EventSelector(ctx, r, updatedSystem, system.KeyConfigurationID, nil)
		assert.Equal(t, proto.TaskType_SYSTEM_LINK.String(), event.Name)
		assert.NoError(t, err)
	})

	t.Run("Should switch on new keyconfig with pkey", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = ptr.PointTo(uuid.New())
		})
		testutils.CreateTestEntities(ctx, t, r, keyConfig)

		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
		})
		updatedSystem := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(uuid.New())
		})

		event, err := m.EventSelector(ctx, r, updatedSystem, system.KeyConfigurationID, keyConfig)
		assert.Equal(t, proto.TaskType_SYSTEM_SWITCH.String(), event.Name)
		assert.NoError(t, err)
	})
}

func TestPatchSystemLinkByID(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	primaryKeyID := uuid.New()
	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = &primaryKeyID
	})

	systems := []*model.System{
		testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
		}),
		testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusFAILED
		}),
		testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusPROCESSING
		}),
	}

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	for i := range systems {
		testutils.CreateTestEntities(ctx, t, r, systems[i])
	}

	system := systems[0]
	systemFailed := systems[1]
	systemProcessing := systems[2]

	t.Run("Should update system link", func(t *testing.T) {
		expected := system
		expected.Status = cmkapi.SystemStatusPROCESSING
		expected.KeyConfigurationName = &keyConfig.Name

		actualSystem, err := m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
		})

		assert.NoError(t, err)
		assert.Equal(t, expected, actualSystem)
	})

	t.Run("Should not be able to update system in failed state", func(t *testing.T) {
		_, err := m.PatchSystemLinkByID(ctx, systemFailed.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
		})

		assert.ErrorIs(t, err, manager.ErrLinkSystemProcessingOrFailed)
	})

	t.Run("Should not be able to update system in processing state", func(t *testing.T) {
		_, err := m.PatchSystemLinkByID(ctx, systemProcessing.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
		})

		assert.ErrorIs(t, err, manager.ErrLinkSystemProcessingOrFailed)
	})

	t.Run("Should fail on updating non-existing system", func(t *testing.T) {
		id := uuid.New()
		actualSystem, err := m.PatchSystemLinkByID(ctx, id, cmkapi.SystemPatch{KeyConfigurationID: keyConfig.ID})

		assert.Nil(t, actualSystem)
		assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
	})

	t.Run("Should fail on updating system", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced).WithUpdate()
		forced.WithUpdate().Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		actualSystem, err := m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
		})

		assert.Nil(t, actualSystem)
		assert.ErrorIs(t, err, manager.ErrUpdateSystem)
	})
}

func TestPatchSystemLinkByID_KeyConfigWithoutPrimary(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = nil
	})
	system := &model.System{
		ID:                 uuid.New(),
		Identifier:         uuid.New().String(),
		KeyConfigurationID: &keyConfig.ID,
	}

	testutils.CreateTestEntities(ctx, t, r, system, keyConfig)
	_, err := m.PatchSystemLinkByID(ctx, system.ID, cmkapi.SystemPatch{
		KeyConfigurationID: keyConfig.ID,
	})

	assert.ErrorIs(t, err, manager.ErrAddSystemNoPrimaryKey)
}

func TestDeleteSystemLinkByID(t *testing.T) {
	logger := testutils.SetupLoggerWithBuffer()
	systemService := systems.NewFakeService(logger)
	_, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
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
	t.Cleanup(func() {
		assert.NoError(t, clientsFactory.Close())
	})

	m, db, tenant := SetupSystemManager(t, clientsFactory)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = ptr.PointTo(uuid.New())
	})
	region := regionpb.Region_REGION_EU.String()
	sysType := string(systems.SystemTypeSYSTEM)

	t.Run("Should delete system link", func(t *testing.T) {
		systemUnderTest := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.New().String(),
			KeyConfigurationID: &keyConfig.ID,
			Region:             region,
			Type:               sysType,
			Status:             cmkapi.SystemStatusCONNECTED,
		}
		testutils.CreateTestEntities(ctx, t, r, systemUnderTest, keyConfig)
		registerSystem(ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type)

		err := m.DeleteSystemLinkByID(ctx, systemUnderTest.ID)
		assert.NoError(t, err)

		res := &model.System{ID: systemUnderTest.ID}

		_, err = r.First(ctx, res, *repo.NewQuery())
		assert.NoError(t, err)

		assert.Nil(t, res.KeyConfigurationID)

		resp, err := systemService.ListSystems(ctx,
			&systemgrpc.ListSystemsRequest{
				ExternalId: systemUnderTest.Identifier,
				Region:     systemUnderTest.Region,
			})
		assert.NoError(t, err)
		assert.False(t, resp.GetSystems()[0].GetHasL1KeyClaim())
	})

	t.Run("Should not delete system link blocked state failed", func(t *testing.T) {
		systemUnderTest := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.New().String(),
			KeyConfigurationID: &keyConfig.ID,
			Region:             region,
			Type:               sysType,
			Status:             cmkapi.SystemStatusFAILED,
		}
		testutils.CreateTestEntities(ctx, t, r, systemUnderTest)
		registerSystem(ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type)

		err := m.DeleteSystemLinkByID(ctx, systemUnderTest.ID)
		assert.ErrorIs(t, err, manager.ErrUnlinkSystemProcessingOrFailed)
	})

	t.Run("Should not delete system link blocked state connecting", func(t *testing.T) {
		systemUnderTest := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.New().String(),
			KeyConfigurationID: &keyConfig.ID,
			Region:             region,
			Type:               sysType,
			Status:             cmkapi.SystemStatusPROCESSING,
		}
		testutils.CreateTestEntities(ctx, t, r, systemUnderTest)
		registerSystem(ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type)

		err := m.DeleteSystemLinkByID(ctx, systemUnderTest.ID)
		assert.ErrorIs(t, err, manager.ErrUnlinkSystemProcessingOrFailed)
	})

	t.Run("Should error on delete system link with empty keyConfigurationID", func(t *testing.T) {
		system := &model.System{
			ID:                 uuid.New(),
			KeyConfigurationID: nil,
		}
		err := r.Create(ctx, system)
		assert.NoError(t, err)

		err = m.DeleteSystemLinkByID(ctx, system.ID)
		assert.ErrorIs(t, err, manager.ErrUpdateSystem)
	})

	t.Run("Should error on delete system link with non-existing system", func(t *testing.T) {
		system := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.New().String(),
			KeyConfigurationID: nil,
		}

		err := m.DeleteSystemLinkByID(ctx, system.ID)
		assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
	})
}

func TestRefreshSystems(t *testing.T) {
	logger := testutils.SetupLoggerWithBuffer()
	systemService := systems.NewFakeService(logger)
	_, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
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
	assert.NoError(t, clientsFactory.Close())

	m, db, tenant := SetupSystemManager(t, clientsFactory)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	existingSystem := &model.System{
		ID:         uuid.New(),
		Identifier: uuid.New().String(),
		Region:     regionpb.Region_REGION_EU.String(),
		Type:       string(systems.SystemTypeSYSTEM),
	}
	testutils.CreateTestEntities(ctx, t, r, existingSystem)

	t.Run("No systems in registry - systems in DB remain unchanged", func(t *testing.T) {
		// Act
		m.RefreshSystemsData(ctx)
		// Verify
		systems := []*model.System{}
		count, err := r.List(ctx, &model.System{}, &systems, *repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().
					Where(
						repo.IdentifierField, existingSystem.Identifier)),
		))
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, existingSystem.ID, systems[0].ID)
	})

	t.Run("No systems for current tenant in registry - systems in DB remain unchanged", func(t *testing.T) {
		// Prepare
		existingSystems := []*model.System{}
		existingSystemsCount, _ := r.List(ctx, &model.System{}, &existingSystems, *repo.NewQuery())

		foreignSystem := &model.System{
			ID:         uuid.New(),
			Identifier: uuid.New().String(),
			Region:     regionpb.Region_REGION_US.String(),
			Type:       string(systems.SystemTypeSUBACCOUNT),
		}

		registerSystem(ctx, t, systemService, foreignSystem.Identifier, foreignSystem.Region, foreignSystem.Type,
			func(req *systemgrpc.RegisterSystemRequest) {
				req.TenantId = "OtherTenant"
			})
		// Act
		m.RefreshSystemsData(ctx)
		// Verify
		systems := []*model.System{}
		count, err := r.List(ctx, &model.System{}, &systems, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, existingSystemsCount, count)
		assert.Equal(t, existingSystems, systems)
	})

	t.Run("System from different tenant is not created, as it is not returned by registry", func(t *testing.T) {
		// Prepare
		externalID := uuid.NewString()
		registerSystem(ctx, t, systemService, externalID, regionpb.Region_REGION_EU.String(),
			string(systems.SystemTypeSYSTEM),
			func(req *systemgrpc.RegisterSystemRequest) {
				req.TenantId = "DIFFERENT_TENANT"
			})
		// Act
		m.RefreshSystemsData(ctx)
		// Verify
		sys := &model.System{}
		found, err := r.First(
			ctx,
			sys,
			*repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().
						Where(repo.IdentifierField, externalID),
				),
			),
		)
		assert.False(t, found)
		assert.Error(t, err)
	})

	t.Run("New system returned by the registry with empty SIS metadata", func(t *testing.T) {
		// Prepare
		externalID := uuid.New().String()
		region := regionpb.Region_REGION_US.String()
		systemType := string(systems.SystemTypeSUBACCOUNT)

		registerSystem(ctx, t, systemService, externalID, region, systemType,
			func(req *systemgrpc.RegisterSystemRequest) {
				req.TenantId = tenant
			})
		// Act
		m.RefreshSystemsData(ctx)
		// Verify
		systems := []*model.System{}
		count, err := r.List(ctx, &model.System{}, &systems, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, 2, count)

		foundSystem := &model.System{}
		_, err = r.First(
			ctx,
			foundSystem,
			*repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().
						Where(repo.IdentifierField, externalID),
				),
			),
		)
		assert.NoError(t, err)
		assert.Equal(t, externalID, foundSystem.Identifier)
		assert.Equal(t, systemType, foundSystem.Type)
		assert.Equal(t, region, foundSystem.Region)
	})

	t.Run("Same System in a different region returned by the registry - two different systems in DB", func(t *testing.T) {
		// Prepare
		existingSystems := []*model.System{}
		existingSystemsCount, _ := r.List(ctx, &model.System{}, &existingSystems, *repo.NewQuery())

		registerSystem(ctx, t, systemService, existingSystem.Identifier, existingSystem.Region, existingSystem.Type,
			func(req *systemgrpc.RegisterSystemRequest) {
				req.TenantId = tenant
			})

		externalID := uuid.New().String()
		region := regionpb.Region_REGION_US.String()
		systemType := string(systems.SystemTypeSUBACCOUNT)

		registerSystem(ctx, t, systemService, externalID, region, systemType,
			func(req *systemgrpc.RegisterSystemRequest) {
				req.TenantId = tenant
			})
		// Act
		m.RefreshSystemsData(ctx)
		// Verify
		systems := []*model.System{}
		count, err := r.List(ctx, &model.System{}, &systems, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, existingSystemsCount+1, count)

		for _, sys := range systems {
			if sys.Identifier == existingSystem.Identifier {
				if sys.Region == existingSystem.Region {
					assert.Equal(t, existingSystem.Type, sys.Type)
					assert.Equal(t, existingSystem.ID, sys.ID)
				} else {
					assert.Equal(t, region, sys.Region)
					assert.Equal(t, systemType, sys.Type)
					assert.NotEqual(t, existingSystem.ID, sys.ID)
				}
			}
		}
	})
}
