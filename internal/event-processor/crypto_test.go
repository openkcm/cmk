package eventprocessor_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/clients/registry/systems"
	"github.tools.sap/kms/cmk/internal/config"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	eventProto "github.tools.sap/kms/cmk/internal/event-processor/proto"
	"github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func setup(t *testing.T) (*eventprocessor.CryptoReconciler, *systems.FakeService, repo.Repo, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Event{},
			&model.System{},
			&model.Key{},
			&model.KeyConfiguration{},
			&model.Group{},
		},
		CreateDatabase: true,
		WithOrbital:    true,
	})
	r := sql.NewRepository(db)

	cfg := &config.Config{
		Database: dbCfg,
	}

	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

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

	eventProcessor, err := eventprocessor.NewCryptoReconciler(
		t.Context(), cfg, r,
		ctlg, clientsFactory,
	)
	assert.NoError(t, err)

	t.Cleanup(func() {
		eventProcessor.CloseAmqpClients(context.Background())
	})

	return eventProcessor, systemService, r, tenants[0]
}

func TestKeyEventCreation(t *testing.T) {
	eventProcessor, _, _, tenant := setup(t)

	tests := []struct {
		name       string
		keyEventFn func(ctx context.Context, keyID string) (orbital.Job, error)
		keyID      string
		tenantID   string
		expErr     error
		expType    string
	}{
		{
			name:       "should return error on missing keyID for key detach",
			keyEventFn: eventProcessor.KeyDetach,
			expErr:     eventprocessor.ErrMissingKeyID,
		},
		{
			name:       "should return error on missing keyID for key enable",
			keyEventFn: eventProcessor.KeyEnable,
			expErr:     eventprocessor.ErrMissingKeyID,
		},
		{
			name:       "should return error on missing keyID for key disable",
			keyEventFn: eventProcessor.KeyDisable,
			expErr:     eventprocessor.ErrMissingKeyID,
		},
		{
			name:       "should return error on missing tenant from ctx for key detach",
			keyEventFn: eventProcessor.KeyDetach,
			keyID:      "keyID",
			expErr:     cmkcontext.ErrExtractTenantID,
		},
		{
			name:       "should return error on missing tenant from ctx for key enable",
			keyEventFn: eventProcessor.KeyEnable,
			keyID:      "keyID",
			expErr:     cmkcontext.ErrExtractTenantID,
		},
		{
			name:       "should return error on missing tenant from ctx for key disable",
			keyEventFn: eventProcessor.KeyDisable,
			keyID:      "keyID",
			expErr:     cmkcontext.ErrExtractTenantID,
		},
		{
			name:       "should create key detach event",
			keyEventFn: eventProcessor.KeyDetach,
			keyID:      "keyID",
			tenantID:   tenant,
			expType:    eventProto.TaskType_KEY_DETACH.String(),
		},
		{
			name:       "should create key enable event",
			keyEventFn: eventProcessor.KeyEnable,
			keyID:      "keyID",
			tenantID:   tenant,
			expType:    eventProto.TaskType_KEY_ENABLE.String(),
		},
		{
			name:       "should create key disable event",
			keyEventFn: eventProcessor.KeyDisable,
			keyID:      "keyID",
			tenantID:   tenant,
			expType:    eventProto.TaskType_KEY_DISABLE.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tt.tenantID)
			job, err := tt.keyEventFn(ctx, tt.keyID)

			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Equal(t, orbital.Job{}, job)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expType, job.Type)
			assert.Equal(t, tt.keyID, job.ExternalID)

			var jobData eventprocessor.KeyActionJobData
			assert.NoError(t, json.Unmarshal(job.Data, &jobData))

			assert.Equal(t, tt.tenantID, jobData.TenantID)
			assert.Equal(t, tt.keyID, jobData.KeyID)
		})
	}
}

func TestSystemEventCreation(t *testing.T) {
	eventProcessor, _, r, tenant := setup(t)

	tests := []struct {
		name          string
		systemEventFn func(ctx context.Context, system *model.System, keyIDTo, keyIDFrom string) (orbital.Job, error)
		systemStatus  cmkapi.SystemStatus
		tenantID      string
		keyIDTo       string
		keyIDFrom     string
		expErr        error
		expType       string
		expStatus     cmkapi.SystemStatus
		assertKeyID   func(t *testing.T, keyIDTo, keyIDFrom string, data eventprocessor.SystemActionJobData)
	}{
		{
			name: "should return error on missing tenant from ctx for system link",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system link",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system link event and set system to processing",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			tenantID:     tenant,
			keyIDTo:      "keyIDTo",
			expType:      eventProto.TaskType_SYSTEM_LINK.String(),
			expStatus:    cmkapi.SystemStatusPROCESSING,
			assertKeyID: func(t *testing.T, keyIDTo, _ string, data eventprocessor.SystemActionJobData) {
				t.Helper()
				assert.Equal(t, keyIDTo, data.KeyIDTo)
			},
		},
		{
			name: "should return error on missing tenant from ctx for system unlink",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system unlink",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from)
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system unlink event and set system to processing",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			tenantID:     tenant,
			keyIDFrom:    "keyIDFrom",
			expType:      eventProto.TaskType_SYSTEM_UNLINK.String(),
			expStatus:    cmkapi.SystemStatusPROCESSING,
			assertKeyID: func(t *testing.T, _, keyIDFrom string, data eventprocessor.SystemActionJobData) {
				t.Helper()
				assert.Equal(t, keyIDFrom, data.KeyIDFrom)
			},
		},
		{
			name: "should return error on missing tenant from ctx for system switch",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system switch",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from)
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system switch event and set system to processing",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			tenantID:     tenant,
			keyIDTo:      "keyIDTo",
			keyIDFrom:    "keyIDFrom",
			expType:      eventProto.TaskType_SYSTEM_SWITCH.String(),
			expStatus:    cmkapi.SystemStatusPROCESSING,
			assertKeyID: func(t *testing.T, keyIDTo, keyIDFrom string, data eventprocessor.SystemActionJobData) {
				t.Helper()
				assert.Equal(t, keyIDTo, data.KeyIDTo)
				assert.Equal(t, keyIDFrom, data.KeyIDFrom)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := testutils.NewSystem(func(s *model.System) {
				s.Status = tt.systemStatus
			})

			ctx := testutils.CreateCtxWithTenant(tt.tenantID)
			if tt.tenantID != "" {
				testutils.CreateTestEntities(ctx, t, r, system)
			}

			job, err := tt.systemEventFn(ctx, system, tt.keyIDTo, tt.keyIDFrom)

			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Equal(t, orbital.Job{}, job)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expType, job.Type)
			assert.Equal(t, system.ID.String(), job.ExternalID)

			var jobData eventprocessor.SystemActionJobData
			assert.NoError(t, json.Unmarshal(job.Data, &jobData))

			assert.Equal(t, tt.tenantID, jobData.TenantID)
			assert.Equal(t, system.ID.String(), jobData.SystemID)
			tt.assertKeyID(t, tt.keyIDTo, tt.keyIDFrom, jobData)

			_, err = r.First(ctx, system, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, tt.expStatus, system.Status)
		})
	}
}

func TestJobConfirmation(t *testing.T) {
	eventProcessor, _, r, tenant := setup(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	tests := []struct {
		name string
		job  func(s *model.System, keyID string) (orbital.Job, error)
	}{
		{
			name: "should confirm system link job",
			job: func(s *model.System, keyID string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, s, keyID)
			},
		},
		{
			name: "should confirm system unlink job",
			job: func(s *model.System, keyID string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, s, keyID)
			},
		},
		{
			name: "should confirm key enable job",
			job: func(_ *model.System, keyID string) (orbital.Job, error) {
				return eventProcessor.KeyEnable(ctx, keyID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := testutils.NewSystem(func(_ *model.System) {})
			key := testutils.NewKey(func(_ *model.Key) {})
			testutils.CreateTestEntities(ctx, t, r, system, key)

			job, err := tt.job(system, key.ID.String())

			assert.NoError(t, err)

			orbitalCtx := cmkcontext.CreateTenantContext(ctx, "orbital")
			jobFromDB := &testutils.OrbitalJob{ID: job.ID}
			_, err = r.First(orbitalCtx, jobFromDB, *repo.NewQuery())
			assert.NoError(t, err)

			assert.Equal(t, job.Type, jobFromDB.Type)
			assert.Equal(t, job.ID.String(), jobFromDB.ID.String())

			assert.NotEqual(t, orbital.JobStatusConfirming, jobFromDB.Status)
		})
	}
}

func TestJobTermination(t *testing.T) {
	eventProcessor, systemService, r, tenant := setup(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	system := testutils.NewSystem(func(_ *model.System) {})
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	testutils.CreateTestEntities(ctx, t, r, system, keyConfig, key)

	jobData := eventprocessor.SystemActionJobData{
		TenantID: tenant,
		SystemID: system.ID.String(),
		KeyIDTo:  key.ID.String(),
	}
	dataBytes, err := json.Marshal(jobData)
	assert.NoError(t, err)

	unlinkJobData := eventprocessor.SystemActionJobData{
		TenantID:  tenant,
		SystemID:  system.ID.String(),
		KeyIDFrom: key.ID.String(),
	}
	unlinkDataBytes, err := json.Marshal(unlinkJobData)
	assert.NoError(t, err)

	t.Run("Should update system key config ID on job termination", func(t *testing.T) {
		_, err := r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Nil(t, system.KeyConfigurationID)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventProto.TaskType_SYSTEM_LINK.String(),
			Data:       dataBytes,
		}, true)

		systemAfterLink := &model.System{
			ID: system.ID,
		}

		_, err = r.First(ctx, systemAfterLink, *repo.NewQuery())
		assert.NoError(t, err)
		assert.NotNil(t, systemAfterLink.KeyConfigurationID)

		item = uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventProto.TaskType_SYSTEM_UNLINK.String(),
			Data:       unlinkDataBytes,
		}, true)

		systemAfterUnlink := &model.System{
			ID: system.ID,
		}

		_, err = r.First(ctx, systemAfterUnlink, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Nil(t, systemAfterUnlink.KeyConfigurationID)
	})

	t.Run("Should update key claim on job termination", func(t *testing.T) {
		req := &systemgrpc.RegisterSystemRequest{
			ExternalId:    system.Identifier,
			L2KeyId:       "key123",
			Region:        "test",
			Type:          "test",
			HasL1KeyClaim: false,
		}

		_, err := systemService.RegisterSystem(ctx, req)
		assert.NoError(t, err)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventProto.TaskType_SYSTEM_LINK.String(),
			Data:       dataBytes,
		}, true)

		resp, err := systemService.ListSystems(ctx,
			&systemgrpc.ListSystemsRequest{
				ExternalId: system.Identifier,
				Region:     "test",
			})
		assert.NoError(t, err)
		assert.True(t, resp.GetSystems()[0].GetHasL1KeyClaim())

		item = uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventProto.TaskType_SYSTEM_UNLINK.String(),
			Data:       dataBytes,
		}, true)

		resp, err = systemService.ListSystems(ctx,
			&systemgrpc.ListSystemsRequest{
				ExternalId: system.Identifier,
				Region:     "test",
			})
		assert.NoError(t, err)
		assert.False(t, resp.GetSystems()[0].GetHasL1KeyClaim())
	})

	t.Run("Should delete item in cmk events db on successful job termination", func(t *testing.T) {
		event := &model.Event{
			Identifier: uuid.NewString(),
			Type:       eventProto.TaskType_SYSTEM_LINK.String(),
			Data:       dataBytes,
			Status:     orbital.JobStatusDone,
		}
		err := r.Create(ctx, event)
		assert.NoError(t, err)

		terminateNewJob(t, eventProcessor, event, true)

		_, err = r.First(ctx, event, *repo.NewQuery())
		assert.ErrorIs(t, err, repo.ErrNotFound)
	})

	t.Run("Should not delete item in cmk events db on unsuccessful job termination", func(t *testing.T) {
		event := &model.Event{
			Identifier: uuid.NewString(),
			Type:       eventProto.TaskType_SYSTEM_LINK.String(),
			Data:       dataBytes,
			Status:     orbital.JobStatusProcessing,
		}
		err := r.Create(ctx, event)
		assert.NoError(t, err)

		terminateNewJob(t, eventProcessor, event, false)

		_, err = r.First(ctx, event, *repo.NewQuery())
		assert.NoError(t, err)
	})
}

func terminateNewJob(
	t *testing.T,
	eventProcessor *eventprocessor.CryptoReconciler,
	e *model.Event,
	jobDone bool,
) {
	t.Helper()

	job := orbital.Job{
		ExternalID: e.Identifier,
		Data:       e.Data,
		Type:       e.Type,
		Status:     e.Status,
	}

	// Ignored as this test is not testing the system update capabilities
	_ = eventProcessor.JobTerminationFunc(t.Context(), job)
	if jobDone {
		job.Status = orbital.JobStatusDone
	}

	err := eventProcessor.JobTerminationFunc(t.Context(), job)
	if err != nil {
		t.Logf("Job termination returned error: %v", err)
	}
}
