package manager_test

import (
	"context"
	"encoding/json"
	"testing"

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

func SetupSystemManager(t *testing.T, clientsFactory clients.Factory) (
	*manager.SystemManager,
	*multitenancy.DB,
	string,
) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

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
	userManager := manager.NewUserManager(dbRepository, auditor.New(t.Context(), &cfg))
	tagManager := manager.NewTagManager(dbRepository)
	keyConfigManager := manager.NewKeyConfigManager(dbRepository, certManager, userManager, tagManager, nil, &cfg)

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
		userManager,
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
	t.Run("Should create system manager", func(t *testing.T) {
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

	t.Run("Should get all systems", func(t *testing.T) {
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
	t.Run("Should fail to get all systems", func(t *testing.T) {
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

	t.Run("Should get all systems filtered by keyConfigID", func(t *testing.T) {
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
	t.Run("Should fail to get all systems filtered by keyConfigID", func(t *testing.T) {
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

	t.Run("Should get all systems filtered by region", func(t *testing.T) {
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

	t.Run("Should get all systems filtered by type", func(t *testing.T) {
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

	t.Run("Should fail to get systems filtered by region", func(t *testing.T) {
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
	t.Run("Should fail to get systems filtered by type", func(t *testing.T) {
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

	t.Run("Should get newSystem by id", func(t *testing.T) {
		actualSystem, err := m.GetSystemByID(ctx, newSystem.ID)

		assert.Equal(t, newSystem, actualSystem)
		assert.NoError(t, err)
	},
	)

	t.Run("Should fail on get newSystem by id", func(t *testing.T) {
		actualSystem, err := m.GetSystemByID(ctx, uuid.New())

		assert.Nil(t, actualSystem)
		assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
	},
	)

	t.Run("Should not get keyconfig name", func(t *testing.T) {
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

func TestGetRecoveryAction(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	t.Run("Should error on system that has not triggered actions", func(t *testing.T) {
		sys := testutils.NewSystem(
			func(s *model.System) {
				s.Status = cmkapi.SystemStatusFAILED
			},
		)
		testutils.CreateTestEntities(ctx, t, r, sys)
		res, err := m.GetRecoveryActions(ctx, sys.ID)
		assert.ErrorIs(t, err, eventprocessor.ErrNoPreviousEvent)
		assert.Equal(t, cmkapi.SystemRecoveryAction{
			CanRetry:  false,
			CanCancel: false,
		}, res)
	})

	t.Run("Should have cancel and retry as false if system status is not failed", func(t *testing.T) {
		sys := testutils.NewSystem(
			func(s *model.System) {
				s.Status = cmkapi.SystemStatusCONNECTED
			},
		)
		testutils.CreateTestEntities(ctx, t, r, sys, &model.Event{
			Identifier:         sys.ID.String(),
			PreviousItemStatus: "",
			Data:               json.RawMessage("{}"),
		})

		res, err := m.GetRecoveryActions(ctx, sys.ID)
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemRecoveryAction{
			CanRetry:  false,
			CanCancel: false,
		}, res)
	})
}

func TestGetRecoveryActionAuthorisation(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	r := sql.NewRepository(db)

	// Create admin and non-admin users
	group := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "admin-group"
	})
	nonAdminGroup := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "non-admin-group"
	})

	// Create two key configs with different admin groups
	primaryKeyID1 := uuid.New()
	primaryKeyID2 := uuid.New()

	keyConfig1 := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = &primaryKeyID1
		k.AdminGroupID = group.ID
		k.AdminGroup = *group
	})

	keyConfig2 := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.PrimaryKeyID = &primaryKeyID2
		k.AdminGroupID = nonAdminGroup.ID
		k.AdminGroup = *nonAdminGroup
	})

	// Create keys for both configs
	key1 := testutils.NewKey(func(k *model.Key) {
		k.ID = primaryKeyID1
		k.KeyConfigurationID = keyConfig1.ID
	})

	key2 := testutils.NewKey(func(k *model.Key) {
		k.ID = primaryKeyID2
		k.KeyConfigurationID = keyConfig2.ID
	})

	adminCtx := testutils.CreateCtxWithTenant(tenant)
	adminCtx = testutils.InjectClientDataIntoContext(adminCtx, "admin-user", []string{"admin-group"})

	nonAdminCtx := testutils.CreateCtxWithTenant(tenant)
	nonAdminCtx = testutils.InjectClientDataIntoContext(nonAdminCtx, "non-admin-user", []string{"non-admin-group"})

	testutils.CreateTestEntities(adminCtx, t, r, keyConfig1, keyConfig2, key1, key2)

	t.Run("SYSTEM_UNLINK: admin can retry and cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID:  sys.ID.String(),
			TenantID:  tenant,
			KeyIDFrom: key1.ID.String(),
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(adminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_UNLINK.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(adminCtx, sys.ID)
		assert.NoError(t, err)
		assert.True(t, res.CanRetry, "Key Admin should be able to retry SYSTEM_UNLINK")
		assert.True(t, res.CanCancel, "SYSTEM_UNLINK should allow cancel")
	})

	t.Run("SYSTEM_UNLINK: non-admin cannot retry but can cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID:  sys.ID.String(),
			TenantID:  tenant,
			KeyIDFrom: key1.ID.String(),
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(nonAdminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_UNLINK.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(nonAdminCtx, sys.ID)
		assert.NoError(t, err)
		assert.False(t, res.CanRetry, "Non-admin should not be able to retry SYSTEM_UNLINK")
		assert.True(t, res.CanCancel, "SYSTEM_UNLINK should allow cancel")
	})

	t.Run("SYSTEM_SWITCH with set_primary_key: admin can retry, cannot cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID:  sys.ID.String(),
			TenantID:  tenant,
			KeyIDTo:   key1.ID.String(),
			KeyIDFrom: key2.ID.String(),
			Trigger:   constants.KeyActionSetPrimary,
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(adminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_SWITCH.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(adminCtx, sys.ID)
		assert.NoError(t, err)
		assert.True(t, res.CanRetry, "Key Admin should be able to retry set_primary_key")
		assert.False(t, res.CanCancel, "set_primary_key should not allow cancel")
	})

	t.Run("SYSTEM_SWITCH with set_primary_key: non-admin cannot retry or cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID:  sys.ID.String(),
			TenantID:  tenant,
			KeyIDTo:   key1.ID.String(),
			KeyIDFrom: key2.ID.String(),
			Trigger:   constants.KeyActionSetPrimary,
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(nonAdminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_SWITCH.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(nonAdminCtx, sys.ID)
		assert.NoError(t, err)
		assert.False(t, res.CanRetry, "Non-admin should not be able to retry set_primary_key")
		assert.False(t, res.CanCancel, "set_primary_key should not allow cancel")
	})

	t.Run("SYSTEM_SWITCH regular: admin can retry and cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID:  sys.ID.String(),
			TenantID:  tenant,
			KeyIDTo:   key2.ID.String(),
			KeyIDFrom: key1.ID.String(),
			Trigger:   "", // Empty trigger = regular switch
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(adminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_SWITCH.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(adminCtx, sys.ID)
		assert.NoError(t, err)
		assert.True(t, res.CanRetry, "Key Admin of SOURCE config should be able to retry switch")
		assert.True(t, res.CanCancel, "Regular switch should allow cancel")
	})

	t.Run("SYSTEM_SWITCH regular: non-admin cannot retry but can cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID:  sys.ID.String(),
			TenantID:  tenant,
			KeyIDTo:   key2.ID.String(),
			KeyIDFrom: key1.ID.String(),
			Trigger:   "", // Empty trigger = regular switch
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(nonAdminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_SWITCH.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(nonAdminCtx, sys.ID)
		assert.NoError(t, err)
		assert.False(t, res.CanRetry, "Non-admin of SOURCE config should not be able to retry switch")
		assert.True(t, res.CanCancel, "Regular switch should allow cancel")
	})

	t.Run("SYSTEM_LINK: admin can retry and cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID: sys.ID.String(),
			TenantID: tenant,
			KeyIDTo:  key1.ID.String(),
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(adminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_LINK.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(adminCtx, sys.ID)
		assert.NoError(t, err)
		assert.True(t, res.CanRetry, "Key Admin should be able to retry SYSTEM_LINK")
		assert.True(t, res.CanCancel, "SYSTEM_LINK should allow cancel")
	})

	t.Run("SYSTEM_LINK: non-admin cannot retry but can cancel", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusFAILED
		})

		jobData := eventprocessor.SystemActionJobData{
			SystemID: sys.ID.String(),
			TenantID: tenant,
			KeyIDTo:  key1.ID.String(),
		}
		data, _ := json.Marshal(jobData)

		testutils.CreateTestEntities(nonAdminCtx, t, r, sys, &model.Event{
			Identifier: sys.ID.String(),
			Type:       proto.TaskType_SYSTEM_LINK.String(),
			Data:       data,
		})

		res, err := m.GetRecoveryActions(nonAdminCtx, sys.ID)
		assert.NoError(t, err)
		assert.False(t, res.CanRetry, "Non-admin should not be able to retry SYSTEM_LINK")
		assert.True(t, res.CanCancel, "SYSTEM_LINK should allow cancel")
	})
}

func TestSendRecoveryAction(t *testing.T) {
	m, db, tenant := SetupSystemManager(t, nil)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group4"})
	r := sql.NewRepository(db)

	t.Run("Should cancel action", func(t *testing.T) {
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

		err := m.SendRecoveryActions(ctx, sys.ID, cmkapi.SystemRecoveryActionBodyActionCANCEL)
		assert.NoError(t, err)

		_, err = r.First(ctx, sys, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusCONNECTED, sys.Status)
	},
	)

	t.Run("Should error on cancel if there are no previous actions", func(t *testing.T) {
		sys := testutils.NewSystem(
			func(s *model.System) {
				s.Status = cmkapi.SystemStatusFAILED
			},
		)

		err := m.SendRecoveryActions(ctx, sys.ID, cmkapi.SystemRecoveryActionBodyActionCANCEL)
		assert.ErrorIs(t, err, repo.ErrNotFound)
	})

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

	t.Run("Should error on retry with system status not failed", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		testutils.CreateTestEntities(ctx, t, r, system)

		_, err := m.LinkSystemAction(
			ctx, system.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
			},
		)
		assert.NoError(t, err)

		err = m.SendRecoveryActions(ctx, system.ID, cmkapi.SystemRecoveryActionBodyActionRETRY)
		assert.ErrorIs(t, err, manager.ErrRetryNonFailedSystem)
	},
	)

	t.Run("Should error on retry without previous event", func(t *testing.T) {
		system := testutils.NewSystem(
			func(s *model.System) {
				s.Status = cmkapi.SystemStatusFAILED
			},
		)
		testutils.CreateTestEntities(ctx, t, r, system)

		err := m.SendRecoveryActions(ctx, system.ID, cmkapi.SystemRecoveryActionBodyActionRETRY)
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

				err := m.SendRecoveryActions(ctx, system.ID, cmkapi.SystemRecoveryActionBodyActionRETRY)
				assert.ErrorIs(t, err, manager.ErrRetryNonFailedSystem)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			err = m.SendRecoveryActions(ctx, system.ID, cmkapi.SystemRecoveryActionBodyActionRETRY)
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

	t.Run("Should LINK if new system", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(
			func(k *model.KeyConfiguration) {
				k.PrimaryKeyID = ptr.PointTo(uuid.New())
			},
		)

		testutils.CreateTestEntities(ctx, t, r, keyConfig)
		system := testutils.NewSystem(func(_ *model.System) {})
		event, err := m.EventSelector(ctx, system, keyConfig)
		assert.Equal(t, proto.TaskType_SYSTEM_LINK.String(), event.Name)
		assert.NoError(t, err)
	})

	t.Run("Should send SWITCH if the old key config doesnt match new key config", func(t *testing.T) {
		// given
		oldKeyConfig := testutils.NewKeyConfig(
			func(k *model.KeyConfiguration) {
				k.PrimaryKeyID = ptr.PointTo(uuid.New())
			},
		)
		newKeyConfig := testutils.NewKeyConfig(
			func(k *model.KeyConfiguration) {
				k.PrimaryKeyID = ptr.PointTo(uuid.New())
			},
		)
		testutils.CreateTestEntities(ctx, t, r, oldKeyConfig, newKeyConfig)

		system := testutils.NewSystem(
			func(s *model.System) {
				s.KeyConfigurationID = ptr.PointTo(oldKeyConfig.ID)
			},
		)

		// when
		event, err := m.EventSelector(ctx, system, newKeyConfig)

		// then
		assert.Equal(t, proto.TaskType_SYSTEM_SWITCH.String(), event.Name)
		assert.NoError(t, err)
	})

	t.Run("Should send LINK if old key config same as new", func(t *testing.T) {
		// given
		keyConfig := testutils.NewKeyConfig(
			func(k *model.KeyConfiguration) {
				k.PrimaryKeyID = ptr.PointTo(uuid.New())
			},
		)
		testutils.CreateTestEntities(ctx, t, r, keyConfig)

		system := testutils.NewSystem(
			func(s *model.System) {
				s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
			},
		)

		// when
		event, err := m.EventSelector(ctx, system, keyConfig)

		// then
		assert.Equal(t, proto.TaskType_SYSTEM_LINK.String(), event.Name)
		assert.NoError(t, err)
	})
}

func TestLinkSystemAction(t *testing.T) {
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
	keyConfig2 := testutils.NewKeyConfig(
		func(k *model.KeyConfiguration) {
			k.PrimaryKeyID = ptr.PointTo(uuid.New())
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

	testutils.CreateTestEntities(ctx, t, r, keyConfig, keyConfig2)

	for i := range allSystems {
		testutils.CreateTestEntities(ctx, t, r, allSystems[i])
	}

	system := allSystems[0]
	systemFailed := allSystems[1]
	systemProcessing := allSystems[2]

	t.Run("Should update system link", func(t *testing.T) {
		expected := system
		expected.Status = cmkapi.SystemStatusPROCESSING
		expected.KeyConfigurationName = &keyConfig.Name

		actualSystem, err := m.LinkSystemAction(
			ctx, system.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
			},
		)

		assert.NoError(t, err)
		assert.Equal(t, expected, actualSystem)
	},
	)

	t.Run("Should not be able to update system in failed state", func(t *testing.T) {
		_, err := m.LinkSystemAction(
			ctx, systemFailed.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
			},
		)

		assert.ErrorIs(t, err, manager.ErrLinkSystemProcessingOrFailed)
	},
	)

	t.Run("Should not be able to update system in processing state", func(t *testing.T) {
		_, err := m.LinkSystemAction(
			ctx, systemProcessing.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
			},
		)

		assert.ErrorIs(t, err, manager.ErrLinkSystemProcessingOrFailed)
	},
	)

	t.Run("Should fail on updating non-existing system", func(t *testing.T) {
		id := uuid.New()
		actualSystem, err := m.LinkSystemAction(ctx, id, cmkapi.SystemPatch{KeyConfigurationID: keyConfig.ID})

		assert.Nil(t, actualSystem)
		assert.ErrorIs(t, err, manager.ErrGettingSystemByID)
	},
	)

	t.Run("Should fail on updating system", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced).WithUpdate()
		forced.WithUpdate().Register()
		t.Cleanup(
			func() {
				forced.Unregister()
			},
		)

		actualSystem, err := m.LinkSystemAction(
			ctx, system.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
			},
		)

		assert.Nil(t, actualSystem)
		assert.ErrorIs(t, err, manager.ErrUpdateSystem)
	},
	)

	t.Run("Should fail on link to keyconfig without pkey", func(t *testing.T) {
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
		_, err := m.LinkSystemAction(
			ctx, system.ID, cmkapi.SystemPatch{
				KeyConfigurationID: keyConfig.ID,
			},
		)

		assert.ErrorIs(t, err, manager.ErrAddSystemNoPrimaryKey)
	})
}

func TestUnlinkSystemAction(t *testing.T) {
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
		registerSystem(
			ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type,
		)
		err := m.UnlinkSystemAction(ctx, systemUnderTest.ID)
		assert.ErrorIs(t, err, manager.ErrUnlinkSystemProcessingOrFailed)
	},
	)

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
		registerSystem(
			ctx, t, systemService, systemUnderTest.Identifier, systemUnderTest.Region, systemUnderTest.Type,
		)
		err := m.UnlinkSystemAction(ctx, systemUnderTest.ID)
		assert.ErrorIs(t, err, manager.ErrUnlinkSystemProcessingOrFailed)
	},
	)

	t.Run("Should error on delete system link with empty keyConfigurationID", func(t *testing.T) {
		system := &model.System{
			ID:                 uuid.New(),
			KeyConfigurationID: nil,
		}
		err := r.Create(ctx, system)
		assert.NoError(t, err)

		err = m.UnlinkSystemAction(ctx, system.ID)
		assert.ErrorIs(t, err, manager.ErrUpdateSystem)
	},
	)

	t.Run("Should error on delete system link with non-existing system", func(t *testing.T) {
		system := &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.New().String(),
			KeyConfigurationID: nil,
		}
		err := m.UnlinkSystemAction(ctx, system.ID)
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

	t.Run("No systems in registry - systems in DB deleted", func(t *testing.T) {
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

	t.Run("No systems for current tenant in registry - systems in DB deleted", func(t *testing.T) {
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

	t.Run("System from different tenant is not created, as it is not returned by registry", func(t *testing.T) {
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

	t.Run("New system returned by the registry with empty SIS metadata", func(t *testing.T) {
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

	t.Run("Same System in a different region returned by the registry - two different systems in DB", func(t *testing.T) {
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
