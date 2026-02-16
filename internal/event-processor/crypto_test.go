package eventprocessor_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	eventProto "github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/clients/registry/mapping"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func setup(t *testing.T, targetRegions []string) (*eventprocessor.CryptoReconciler, *systems.FakeService, repo.Repo, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	r := sql.NewRepository(db)

	cfg := &config.Config{
		Database: dbCfg,
	}
	if len(targetRegions) > 0 {
		rabbitMQURL := testutils.StartRabbitMQ(t)
		cfg.EventProcessor.Targets = make([]config.Target, 0, len(targetRegions))
		for _, region := range targetRegions {
			cfg.EventProcessor.Targets = append(cfg.EventProcessor.Targets, config.Target{
				Region: region,
				AMQP: config.AMQP{
					URL:    rabbitMQURL,
					Target: region,
					Source: region,
				},
			})
		}
	}

	svcRegistry, err := cmkpluginregistry.New(t.Context(), cfg)
	assert.NoError(t, err)

	logger := testutils.SetupLoggerWithBuffer()
	systemService := systems.NewFakeService(logger)
	mappingService := mapping.NewFakeService()
	_, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
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

	eventProcessor, err := eventprocessor.NewCryptoReconciler(
		t.Context(), cfg, r,
		svcRegistry, clientsFactory,
	)
	assert.NoError(t, err)

	t.Cleanup(func() {
		eventProcessor.CloseAmqpClients(context.Background())
	})

	return eventProcessor, systemService, r, tenants[0]
}

func TestKeyEventCreation(t *testing.T) {
	eventProcessor, _, _, tenant := setup(t, []string{})

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
	eventProcessor, _, r, tenant := setup(t, []string{})

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
			name: "should return error on system in processing state for system link - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system link event and set system to processing - KMS20-3467",
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
				return eventProcessor.SystemUnlink(ctx, system, from, "")
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system unlink - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from, "")
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system unlink event and set system to processing - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from, "")
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
				return eventProcessor.SystemSwitch(ctx, system, to, from, "")
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system switch - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from, "")
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system switch event and set system to processing - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from, "")
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
	eventProcessor, _, r, tenant := setup(t, []string{})
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
				return eventProcessor.SystemUnlink(ctx, s, keyID, "")
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

func TestResolveTasks_KeyTask(t *testing.T) {
	// given
	keyConfigID := uuid.New()
	connectedSystem := &model.System{
		ID:                 uuid.New(),
		Identifier:         "system-connected",
		Region:             "region-connected",
		KeyConfigurationID: &keyConfigID,
		Status:             cmkapi.SystemStatusCONNECTED,
	}
	disconnectedSystem := &model.System{
		ID:                 uuid.New(),
		Identifier:         "system-disconnected",
		Region:             "region-disconnected",
		KeyConfigurationID: &keyConfigID,
		Status:             cmkapi.SystemStatusDISCONNECTED,
	}
	targetlessSystem := &model.System{
		ID:                 uuid.New(),
		Identifier:         "system-targetless",
		Region:             "region-targetless",
		KeyConfigurationID: &keyConfigID,
		Status:             cmkapi.SystemStatusCONNECTED,
	}
	keylessSystem := &model.System{
		ID:         uuid.New(),
		Identifier: "system-keyless",
		Region:     "region-keyless",
		Status:     cmkapi.SystemStatusCONNECTED,
	}

	systemlessTarget := "target-systemless"

	reconciler, _, r, tenant := setup(t, []string{
		connectedSystem.Region,
		disconnectedSystem.Region,
		keylessSystem.Region,
		systemlessTarget,
	})
	resolveTaskFn := reconciler.ResolveTasks()

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	for _, sys := range []*model.System{connectedSystem, disconnectedSystem, targetlessSystem, keylessSystem} {
		err := r.Create(cmkcontext.CreateTenantContext(t.Context(), tenant), sys)
		assert.NoError(t, err)
	}

	t.Run("should return correct targets for", func(t *testing.T) {
		tests := []struct {
			name       string
			taskType   string
			expTargets []string
		}{
			{
				name:       "KEY_ENABLE task",
				taskType:   eventProto.TaskType_KEY_ENABLE.String(),
				expTargets: []string{connectedSystem.Region},
			},
			{
				name:       "KEY_DISABLE task",
				taskType:   eventProto.TaskType_KEY_DISABLE.String(),
				expTargets: []string{connectedSystem.Region},
			},
			{
				name:       "KEY_DETACH task",
				taskType:   eventProto.TaskType_KEY_DETACH.String(),
				expTargets: []string{connectedSystem.Region, disconnectedSystem.Region, keylessSystem.Region, systemlessTarget},
			},
			{
				name:       "KEY_DELETE task",
				taskType:   eventProto.TaskType_KEY_DELETE.String(),
				expTargets: []string{connectedSystem.Region, disconnectedSystem.Region, keylessSystem.Region, systemlessTarget},
			},
			{
				name:       "KEY_ROTATE task",
				taskType:   eventProto.TaskType_KEY_ROTATE.String(),
				expTargets: []string{connectedSystem.Region, disconnectedSystem.Region, keylessSystem.Region, systemlessTarget},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				keyID := uuid.New()
				err := r.Create(ctx, &model.Key{
					ID:                 keyID,
					KeyConfigurationID: keyConfigID,
					Name:               uuid.NewString(),
				})
				assert.NoError(t, err)

				data := eventprocessor.KeyActionJobData{
					TenantID: tenant,
					KeyID:    keyID.String(),
				}
				dataBytes, err := json.Marshal(data)
				assert.NoError(t, err)
				j := orbital.NewJob(tt.taskType, dataBytes)

				// when
				result, err := resolveTaskFn(t.Context(), j, "")

				// then
				assert.NoError(t, err)
				assert.Empty(t, result.CanceledErrorMessage)
				assert.True(t, result.Done)

				actTargets := make([]string, 0, len(result.TaskInfos))
				for _, ti := range result.TaskInfos {
					assert.Equal(t, tt.taskType, ti.Type)

					actData := eventProto.Data{}
					err = proto.Unmarshal(ti.Data, &actData)
					assert.NoError(t, err)

					keyAction := actData.GetKeyAction()
					assert.NotNil(t, keyAction)
					assert.Equal(t, keyID.String(), keyAction.GetKeyId())
					assert.Equal(t, tenant, keyAction.GetTenantId())

					actTargets = append(actTargets, ti.Target)
				}

				assert.ElementsMatch(t, tt.expTargets, actTargets)
			})
		}
	})

	t.Run("should cancel task for", func(t *testing.T) {
		tests := []struct {
			name          string
			data          func() []byte
			cancelMessage string
		}{
			{
				name: "invalid JSON data",
				data: func() []byte {
					return []byte("{invalid-json}")
				},
				cancelMessage: "failed to unmarshal job data",
			},
			{
				name: "missing tenant ID",
				data: func() []byte {
					keyID := uuid.New()
					err := r.Create(ctx, &model.Key{
						ID:                 keyID,
						KeyConfigurationID: uuid.New(),
						Name:               uuid.NewString(),
					})
					assert.NoError(t, err)

					data := eventprocessor.KeyActionJobData{
						KeyID: keyID.String(),
					}
					dataBytes, err := json.Marshal(data)
					assert.NoError(t, err)
					return dataBytes
				},
				cancelMessage: "record not found",
			},
			{
				name: "missing key ID",
				data: func() []byte {
					data := eventprocessor.KeyActionJobData{
						TenantID: tenant,
					}
					dataBytes, err := json.Marshal(data)
					assert.NoError(t, err)
					return dataBytes
				},
				cancelMessage: "failed to get key by ID",
			},
			{
				name: "no matching region targets",
				data: func() []byte {
					keyID := uuid.New()
					err := r.Create(ctx, &model.Key{
						ID:                 keyID,
						KeyConfigurationID: uuid.New(),
						Name:               uuid.NewString(),
					})
					assert.NoError(t, err)

					data := eventprocessor.KeyActionJobData{
						TenantID: tenant,
						KeyID:    keyID.String(),
					}
					dataBytes, err := json.Marshal(data)
					assert.NoError(t, err)
					return dataBytes
				},
				cancelMessage: "no connected regions found for key",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := orbital.NewJob(eventProto.TaskType_KEY_ENABLE.String(), tt.data())

				// when
				result, err := resolveTaskFn(ctx, j, "")

				// then
				assert.NoError(t, err)
				assert.True(t, result.IsCanceled)
				assert.Empty(t, result.TaskInfos)
				assert.Contains(t, result.CanceledErrorMessage, tt.cancelMessage)
			})
		}
	})
}

func TestJobTermination(t *testing.T) {
	eventProcessor, systemService, r, tenant := setup(t, []string{})
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
