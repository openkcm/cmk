package manager_test

import (
	"context"
	"encoding/json"
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

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/clients/registry/systems"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/event-processor/proto"
	"github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	"github.tools.sap/kms/cmk/utils/ptr"
)

func SetupSystemManager(t *testing.T, clientsFactory clients.Factory) (
	*manager.SystemManager,
	*multitenancy.DB,
	string,
) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(
		t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{
				&model.System{},
				&model.KeyConfiguration{},
				&model.SystemProperty{},
				&model.Event{},
				&model.Group{},
			},
		},
	)

	cfg := config.Config{
		Plugins: testutils.SetupMockPlugins(testutils.SystemInfo),
		BaseConfig: commoncfg.BaseConfig{
			Audit: commoncfg.Audit{
				Endpoint: "http://localhost:4318/v1/logs",
			},
		},
		Database: dbCfg,
	}

	ctlg, err := catalog.New(t.Context(), &cfg)
	require.NoError(t, err)

	dbRepository := sql.NewRepository(db)

	if clientsFactory == nil {
		clientsFactory, err = clients.NewFactory(config.Services{})
		assert.NoError(t, err)
	}

	t.Cleanup(
		func() {
			assert.NoError(t, clientsFactory.Close())
		},
	)

	certManager := manager.NewCertificateManager(
		t.Context(), dbRepository, ctlg,
		&config.Certificates{ValidityDays: config.MinCertificateValidityDays},
	)
	keyConfigManager := manager.NewKeyConfigManager(dbRepository, certManager, nil, &cfg)

	eventProcessor, err := eventprocessor.NewCryptoReconciler(
		t.Context(), &cfg, dbRepository,
		ctlg, clientsFactory,
	)
	require.NoError(t, err)

	systemManager := manager.NewSystemManager(
		t.Context(), dbRepository,
		clientsFactory,
		eventProcessor, ctlg,
		&cfg,
		keyConfigManager,
	)

	return systemManager, db, tenants[0]
}

func registerSystem(
	ctx context.Context, t *testing.T, systemService *systems.FakeService, externalID, region,
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
	t.Run(
		"Should create system manager", func(t *testing.T) {
			m, _, _ := SetupSystemManager(t, nil)

			assert.NotNil(t, m)
		},
	)
}

func TestGetAllSystems(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group1"})
	r := sql.NewRepository(db)

	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group1"
		},
	)
	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)

	system1 := testutils.NewSystem(
		func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.KeyConfigurationName = &keyConfig.Name
			s.Properties = map[string]string{
				"a": "b",
				"b": "c",
			}
		},
	)
	system2 := testutils.NewSystem(func(_ *model.System) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		system1,
		system2,
		keyConfig,
	)

	t.Run(
		"Should get all systems", func(t *testing.T) {
			expected := []*model.System{system1, system2}
			filter := manager.SystemFilter{
				Skip: constants.DefaultSkip,
				Top:  constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)
			assert.NoError(t, err)
			assert.ElementsMatch(t, expected, allSystems)
			assert.Equal(t, len(expected), total)
		},
	)
	t.Run(
		"Should fail to get all systems", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)
			forced.Register()
			t.Cleanup(
				func() {
					forced.Unregister()
				},
			)

			filter := manager.SystemFilter{
				Skip: constants.DefaultSkip,
				Top:  constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)
			assert.Nil(t, allSystems)
			assert.Equal(t, 0, total)
			assert.ErrorIs(t, err, manager.ErrQuerySystemList)
		},
	)

	t.Run(
		"Should get all systems filtered by keyConfigID", func(t *testing.T) {
			expected := []*model.System{system1}

			filter := manager.SystemFilter{
				KeyConfigID: keyConfig.ID,
				Skip:        constants.DefaultSkip,
				Top:         constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)
			assert.NoError(t, err)
			assert.Equal(t, expected, allSystems)
			assert.Equal(t, len(expected), total)
		},
	)
	t.Run(
		"Should fail to get all systems filtered by keyConfigID", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)
			forced.Register()
			t.Cleanup(
				func() {
					forced.Unregister()
				},
			)

			filter := manager.SystemFilter{
				KeyConfigID: keyConfig.ID,
				Skip:        constants.DefaultSkip,
				Top:         constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)
			assert.Nil(t, allSystems)
			assert.Equal(t, 0, total)
			assert.ErrorIs(t, err, manager.ErrKeyConfigurationNotFound)
		},
	)
}

func TestGetAllSystemsFiltered(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
	r := sql.NewRepository(db)

	system1 := testutils.NewSystem(
		func(s *model.System) {
			s.Region = "Region1"
			s.Type = "Type1"
		},
	)

	expected1 := []*model.System{system1}

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		system1,
	)

	t.Run(
		"Should get all systems filtered by region", func(t *testing.T) {
			filter := manager.SystemFilter{
				Region: "Region1",
				Skip:   constants.DefaultSkip,
				Top:    constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)

			assert.NoError(t, err)
			assert.Equal(t, expected1, allSystems)
			assert.Equal(t, 1, total)
		},
	)

	t.Run(
		"Should get all systems filtered by type", func(t *testing.T) {
			filter := manager.SystemFilter{
				Type: "Type1",
				Skip: constants.DefaultSkip,
				Top:  constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)

			assert.NoError(t, err)
			assert.Equal(t, expected1, allSystems)
			assert.Equal(t, 1, total)
		},
	)

	t.Run(
		"Should fail to get systems filtered by region", func(t *testing.T) {
			filter := manager.SystemFilter{
				Region: "RegionInvalid",
				Skip:   constants.DefaultSkip,
				Top:    constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)
			assert.NoError(t, err)
			assert.Nil(t, allSystems)
			assert.Equal(t, 0, total)
		},
	)
	t.Run(
		"Should fail to get systems filtered by type", func(t *testing.T) {
			filter := manager.SystemFilter{
				Type: "TypeInvalid",
				Skip: constants.DefaultSkip,
				Top:  constants.DefaultTop,
			}
			allSystems, total, err := m.GetAllSystems(ctx, filter)
			assert.NoError(t, err)
			assert.Nil(t, allSystems)
			assert.Equal(t, 0, total)
		},
	)
}

func TestGetSystemByID(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group2"})
	r := sql.NewRepository(db)

	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group2"
		},
	)
	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)
	newSystem := testutils.NewSystem(
		func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.KeyConfigurationName = &keyConfig.Name
			s.Properties = map[string]string{
				"a": "b",
				"b": "c",
			}
		},
	)

	testutils.CreateTestEntities(ctx, t, r, newSystem, keyConfig)

	t.Run(
		"Should get newSystem by id", func(t *testing.T) {
			actualSystem, err := m.GetSystemByID(ctx, newSystem.ID)

			assert.Equal(t, newSystem, actualSystem)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should fail on get newSystem by id", func(t *testing.T) {
			actualSystem, err := m.GetSystemByID(ctx, uuid.New())

			assert.Nil(t, actualSystem)
			assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
		},
	)

	t.Run(
		"Should not get keyconfig name", func(t *testing.T) {
			system := testutils.NewSystem(
				func(s *model.System) {
					s.KeyConfigurationID = ptr.PointTo(uuid.New())
				},
			)

			testutils.CreateTestEntities(ctx, t, r, system)
			actualSystem, err := m.GetSystemByID(ctx, system.ID)
			assert.Nil(t, actualSystem)
			assert.Error(t, err)
		},
	)
}

func TestGetSystemLinkByID(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group3"})
	r := sql.NewRepository(db)

	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group3"
		},
	)
	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)

	system := &model.System{
		ID:                 uuid.New(),
		Identifier:         uuid.New().String(),
		KeyConfigurationID: &keyConfig.ID,
	}

	testutils.CreateTestEntities(ctx, t, r, system, keyConfig)

	t.Run(
		"Should get system link by id", func(t *testing.T) {
			actualSystem, err := m.GetSystemLinkByID(ctx, system.ID)

			assert.Equal(t, system.KeyConfigurationID, actualSystem)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should fail on getting system by id", func(t *testing.T) {
			link, err := m.GetSystemLinkByID(ctx, uuid.New())

			assert.Nil(t, link)
			assert.ErrorIs(t, err, manager.ErrGettingSystemLinkByID)
		},
	)

	t.Run(
		"Should fail getting system by id with empty keyconfig", func(t *testing.T) {
			system := &model.System{
				ID:                 uuid.New(),
				Identifier:         uuid.New().String(),
				KeyConfigurationID: nil,
			}

			testutils.CreateTestEntities(ctx, t, r, system)
			link, err := m.GetSystemLinkByID(ctx, system.ID)

			assert.Nil(t, link)
			assert.ErrorIs(t, err, manager.ErrKeyConfigurationIDNotFound)
		},
	)
}

func TestCancelSystemAction(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	t.Run(
		"Should cancel action", func(t *testing.T) {
			sys := testutils.NewSystem(
				func(s *model.System) {
					s.Status = cmkapi.SystemStatusFAILED
				},
			)
			testutils.CreateTestEntities(
				ctx, t, r, sys, &model.Event{
					Identifier:         sys.ID.String(),
					PreviousItemStatus: string(cmkapi.SystemStatusCONNECTED),
					Data:               json.RawMessage("{}"),
				},
			)

			err := m.CancelSystemAction(ctx, sys.ID)
			assert.NoError(t, err)

			_, err = r.First(ctx, sys, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, cmkapi.SystemStatusCONNECTED, sys.Status)
		},
	)

	t.Run(
		"Should error if there are no previous actions", func(t *testing.T) {
			sys := testutils.NewSystem(
				func(s *model.System) {
					s.Status = cmkapi.SystemStatusFAILED
				},
			)

			err := m.CancelSystemAction(ctx, sys.ID)
			assert.ErrorIs(t, err, repo.ErrNotFound)
		},
	)
}

func TestEventRetry(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group4"})
	r := sql.NewRepository(db)

	primaryKeyID := uuid.New()
	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group4"
		},
	)
	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = &primaryKeyID
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run(
		"Should error on retry with system status not failed", func(t *testing.T) {
			system := testutils.NewSystem(func(_ *model.System) {})
			testutils.CreateTestEntities(ctx, t, r, system)

			_, err := m.PatchSystemLinkByID(
				ctx, system.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
				},
			)
			assert.NoError(t, err)

			_, err = m.PatchSystemLinkByID(
				ctx, system.ID, cmkapi.SystemPatch{
					Retry: ptr.PointTo(true),
				},
			)
			assert.ErrorIs(t, err, manager.ErrRetryNonFailedSystem)
		},
	)

	t.Run(
		"Should error on retry without previous event", func(t *testing.T) {
			system := testutils.NewSystem(
				func(s *model.System) {
					s.Status = cmkapi.SystemStatusFAILED
				},
			)
			testutils.CreateTestEntities(ctx, t, r, system)

			_, err := m.PatchSystemLinkByID(
				ctx, system.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
					Retry:              ptr.PointTo(true),
				},
			)
			assert.ErrorIs(t, err, eventprocessor.ErrNoPreviousEvent)
		},
	)

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

				_, err := m.PatchSystemLinkByID(
					ctx, system.ID, cmkapi.SystemPatch{
						KeyConfigurationID: keyConfig.ID,
						Retry:              ptr.PointTo(true),
					},
				)
				assert.ErrorIs(t, err, manager.ErrRetryNonFailedSystem)
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				system := testutils.NewSystem(
					func(s *model.System) {
						s.Status = cmkapi.SystemStatusFAILED
					},
				)
				testutils.CreateTestEntities(ctx, t, r, system)
				// Represent task failure
				err := r.Create(
					ctx, &model.Event{
						Identifier: system.ID.String(),
						Type:       tt.eventType.String(),
						Data:       []byte("{}"),
					},
				)
				assert.NoError(t, err)
				err = db.WithTenant(
					ctx, "orbital", func(tx *multitenancy.DB) error {
						job := orbital.Job{
							ID:         uuid.New(),
							ExternalID: system.ID.String(),
							Data:       []byte("{}"),
							Type:       tt.eventType.String(),
							Status:     orbital.JobStatusFailed,
						}

						return tx.Table("jobs").Create(&job).Error
					},
				)
				assert.NoError(t, err)
				_, err = m.PatchSystemLinkByID(
					ctx, system.ID, cmkapi.SystemPatch{
						KeyConfigurationID: keyConfig.ID,
						Retry:              ptr.PointTo(true),
					},
				)
				assert.NoError(t, err)
				tt.f(t, ctx, system)
			},
		)
	}
}

func TestEventSelector(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
	r := sql.NewRepository(db)

	t.Run(
		"Should link on new keyconfig without pkey", func(t *testing.T) {
			system := testutils.NewSystem(func(_ *model.System) {})
			updatedSystem := testutils.NewSystem(func(_ *model.System) {})

			event, err := m.EventSelector(ctx, r, updatedSystem, system.KeyConfigurationID, nil)
			assert.Equal(t, proto.TaskType_SYSTEM_LINK.String(), event.Name)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should switch on new keyconfig with pkey", func(t *testing.T) {
			keyConfig := testutils.NewKeyConfig(
				func(k *model.KeyConfiguration) {
					k.PrimaryKeyID = ptr.PointTo(uuid.New())
				},
			)
			testutils.CreateTestEntities(ctx, t, r, keyConfig)

			system := testutils.NewSystem(
				func(s *model.System) {
					s.KeyConfigurationID = &keyConfig.ID
				},
			)
			updatedSystem := testutils.NewSystem(
				func(s *model.System) {
					s.KeyConfigurationID = ptr.PointTo(uuid.New())
				},
			)
			event, err := m.EventSelector(ctx, r, updatedSystem, system.KeyConfigurationID, keyConfig)
			assert.Equal(t, proto.TaskType_SYSTEM_SWITCH.String(), event.Name)
			assert.NoError(t, err)
		},
	)
}

func TestPatchSystemLinkByID(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
	r := sql.NewRepository(db)

	primaryKeyID := uuid.New()
	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group"
		},
	)
	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = &primaryKeyID
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)

	allSystems := []*model.System{
		testutils.NewSystem(
			func(s *model.System) {
				s.KeyConfigurationID = &keyConfig.ID
			},
		),
		testutils.NewSystem(
			func(s *model.System) {
				s.KeyConfigurationID = &keyConfig.ID
				s.Status = cmkapi.SystemStatusFAILED
			},
		),
		testutils.NewSystem(
			func(s *model.System) {
				s.KeyConfigurationID = &keyConfig.ID
				s.Status = cmkapi.SystemStatusPROCESSING
			},
		),
	}

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	for i := range allSystems {
		testutils.CreateTestEntities(ctx, t, r, allSystems[i])
	}

	system := allSystems[0]
	systemFailed := allSystems[1]
	systemProcessing := allSystems[2]

	t.Run(
		"Should update system link", func(t *testing.T) {
			expected := system
			expected.Status = cmkapi.SystemStatusPROCESSING
			expected.KeyConfigurationName = &keyConfig.Name

			actualSystem, err := m.PatchSystemLinkByID(
				ctx, system.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
				},
			)

			assert.NoError(t, err)
			assert.Equal(t, expected, actualSystem)
		},
	)

	t.Run(
		"Should not be able to update system in failed state", func(t *testing.T) {
			_, err := m.PatchSystemLinkByID(
				ctx, systemFailed.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
				},
			)

			assert.ErrorIs(t, err, manager.ErrLinkSystemProcessingOrFailed)
		},
	)

	t.Run(
		"Should not be able to update system in processing state", func(t *testing.T) {
			_, err := m.PatchSystemLinkByID(
				ctx, systemProcessing.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
				},
			)

			assert.ErrorIs(t, err, manager.ErrLinkSystemProcessingOrFailed)
		},
	)

	t.Run(
		"Should fail on updating non-existing system", func(t *testing.T) {
			id := uuid.New()
			actualSystem, err := m.PatchSystemLinkByID(ctx, id, cmkapi.SystemPatch{KeyConfigurationID: keyConfig.ID})

			assert.Nil(t, actualSystem)
			assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
		},
	)

	t.Run(
		"Should fail on updating system", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced).WithUpdate()
			forced.WithUpdate().Register()
			t.Cleanup(
				func() {
					forced.Unregister()
				},
			)

			actualSystem, err := m.PatchSystemLinkByID(
				ctx, system.ID, cmkapi.SystemPatch{
					KeyConfigurationID: keyConfig.ID,
				},
			)

			assert.Nil(t, actualSystem)
			assert.ErrorIs(t, err, manager.ErrUpdateSystem)
		},
	)
}

func TestPatchSystemLinkByID_KeyConfigWithoutPrimary(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
	r := sql.NewRepository(db)

	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group"
		},
	)

	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = nil
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)
	system := testutils.NewSystem(
		func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.KeyConfigurationName = &keyConfig.Name
		},
	)

	testutils.CreateTestEntities(ctx, t, r, system, keyConfig)
	_, err := m.PatchSystemLinkByID(
		ctx, system.ID, cmkapi.SystemPatch{
			KeyConfigurationID: keyConfig.ID,
		},
	)

	assert.ErrorIs(t, err, manager.ErrAddSystemNoPrimaryKey)
}

func TestDeleteSystemLinkByID(t *testing.T) {
	logger := testutils.SetupLoggerWithBuffer()
	systemService := systems.NewFakeService(logger)
	_, grpcClient := testutils.NewGRPCSuite(
		t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
		},
	)

	clientsFactory, err := clients.NewFactory(
		config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: grpcClient.Target(),
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		},
	)
	assert.NoError(t, err)
	t.Cleanup(
		func() {
			assert.NoError(t, clientsFactory.Close())
		},
	)

	m, db, tenant := SetupSystemManager(t, clientsFactory)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group2"})
	r := sql.NewRepository(db)

	testGroup := testutils.NewGroup(
		func(g *model.Group) {
			g.IAMIdentifier = "test-group2"
		},
	)

	keyConfig := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = ptr.PointTo(uuid.New())
			k.AdminGroupID = testGroup.ID
			k.AdminGroup = *testGroup
		},
	)
	region := regionpb.Region_REGION_EU.String()
	sysType := string(systems.SystemTypeSYSTEM)

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run(
		"Should not delete system link blocked state failed", func(t *testing.T) {
			systemUnderTest := &model.System{
				ID:                 uuid.New(),
				Identifier:         uuid.New().String(),
				KeyConfigurationID: &keyConfig.ID,
				Region:             region,
				Type:               sysType,
				Status:             cmkapi.SystemStatusFAILED,
			}
			testutils.CreateTestEntities(ctx, t, r, systemUnderTest)
			registerSystem(
				ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type,
			)
			err := m.DeleteSystemLinkByID(ctx, systemUnderTest.ID)
			assert.ErrorIs(t, err, manager.ErrUnlinkSystemProcessingOrFailed)
		},
	)

	t.Run(
		"Should not delete system link blocked state connecting", func(t *testing.T) {
			systemUnderTest := &model.System{
				ID:                 uuid.New(),
				Identifier:         uuid.New().String(),
				KeyConfigurationID: &keyConfig.ID,
				Region:             region,
				Type:               sysType,
				Status:             cmkapi.SystemStatusPROCESSING,
			}
			testutils.CreateTestEntities(ctx, t, r, systemUnderTest)
			registerSystem(
				ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type,
			)
			err := m.DeleteSystemLinkByID(ctx, systemUnderTest.ID)
			assert.ErrorIs(t, err, manager.ErrUnlinkSystemProcessingOrFailed)
		},
	)

	t.Run(
		"Should error on delete system link with empty keyConfigurationID", func(t *testing.T) {
			system := &model.System{
				ID:                 uuid.New(),
				KeyConfigurationID: nil,
			}
			err := r.Create(ctx, system)
			assert.NoError(t, err)

			err = m.DeleteSystemLinkByID(ctx, system.ID)
			assert.ErrorIs(t, err, manager.ErrUpdateSystem)
		},
	)

	t.Run(
		"Should error on delete system link with non-existing system", func(t *testing.T) {
			system := &model.System{
				ID:                 uuid.New(),
				Identifier:         uuid.New().String(),
				KeyConfigurationID: nil,
			}
			err := m.DeleteSystemLinkByID(ctx, system.ID)
			assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
		},
	)
}

func TestRefreshSystems(t *testing.T) {
	logger := testutils.SetupLoggerWithBuffer()
	systemService := systems.NewFakeService(logger)
	_, grpcClient := testutils.NewGRPCSuite(
		t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
		},
	)

	clientsFactory, err := clients.NewFactory(
		config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: grpcClient.Target(),
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		},
	)
	assert.NoError(t, err)
	assert.NoError(t, clientsFactory.Close())

	m, db, tenant := SetupSystemManager(t, clientsFactory)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
	r := sql.NewRepository(db)

	existingSystem := &model.System{
		ID:         uuid.New(),
		Identifier: uuid.New().String(),
		Region:     regionpb.Region_REGION_EU.String(),
		Type:       string(systems.SystemTypeSYSTEM),
	}
	testutils.CreateTestEntities(ctx, t, r, existingSystem)

	t.Run(
		"No systems in registry - systems in DB deleted", func(t *testing.T) {
			// Act
			m.RefreshSystemsData(ctx)
			// Verify
			var allSystems []*model.System

			count, err := r.List(
				ctx, &model.System{}, &allSystems, *repo.NewQuery().Where(
					repo.NewCompositeKeyGroup(
						repo.NewCompositeKey().
							Where(
								repo.IdentifierField, existingSystem.Identifier,
							),
					),
				),
			)
			assert.NoError(t, err)
			assert.Equal(t, 0, count)
		},
	)

	t.Run(
		"No systems for current tenant in registry - systems in DB deleted", func(t *testing.T) {
			// Prepare
			foreignSystem := &model.System{
				ID:         uuid.New(),
				Identifier: uuid.New().String(),
				Region:     regionpb.Region_REGION_US.String(),
				Type:       string(systems.SystemTypeSUBACCOUNT),
			}
			registerSystem(
				ctx, t, systemService, foreignSystem.Identifier, foreignSystem.Region, foreignSystem.Type,
				func(req *systemgrpc.RegisterSystemRequest) {
					req.TenantId = "OtherTenant"
				},
			)
			// Act
			m.RefreshSystemsData(ctx)
			// Verify
			var allSystems []*model.System

			count, err := r.List(ctx, &model.System{}, &allSystems, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, 0, count)
		},
	)

	t.Run(
		"System from different tenant is not created, as it is not returned by registry", func(t *testing.T) {
			// Prepare
			externalID := uuid.NewString()
			registerSystem(
				ctx, t, systemService, externalID, regionpb.Region_REGION_EU.String(),
				string(systems.SystemTypeSYSTEM),
				func(req *systemgrpc.RegisterSystemRequest) {
					req.TenantId = "DIFFERENT_TENANT"
				},
			)
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
		},
	)

	t.Run(
		"New system returned by the registry with empty SIS metadata", func(t *testing.T) {
			externalID := uuid.New().String()
			region := regionpb.Region_REGION_US.String()
			systemType := string(systems.SystemTypeSUBACCOUNT)
			registerSystem(
				ctx, t, systemService, externalID, region, systemType,
				func(req *systemgrpc.RegisterSystemRequest) {
					req.TenantId = tenant
				},
			)
			m.RefreshSystemsData(ctx)

			var allSystems []*model.System

			count, err := r.List(ctx, &model.System{}, &allSystems, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, 1, count)

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
		},
	)

	t.Run(
		"Same System in a different region returned by the registry - two different systems in DB", func(t *testing.T) {
			// Prepare
			testutils.CreateTestEntities(ctx, t, r, existingSystem)

			existingSystems := []*model.System{}
			existingSystemsCount, _ := r.List(ctx, &model.System{}, &existingSystems, *repo.NewQuery())
			registerSystem(
				ctx, t, systemService, existingSystem.Identifier, existingSystem.Region, existingSystem.Type,
				func(req *systemgrpc.RegisterSystemRequest) {
					req.TenantId = tenant
				},
			)

			externalID := uuid.New().String()
			region := regionpb.Region_REGION_US.String()
			systemType := string(systems.SystemTypeSUBACCOUNT)
			registerSystem(
				ctx, t, systemService, externalID, region, systemType,
				func(req *systemgrpc.RegisterSystemRequest) {
					req.TenantId = tenant
				},
			)
			m.RefreshSystemsData(ctx)

			var allSystems []*model.System

			count, err := r.List(ctx, &model.System{}, &allSystems, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, existingSystemsCount+1, count)

			for _, sys := range allSystems {
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
		},
	)
}
