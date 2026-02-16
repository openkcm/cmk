package eventprocessor_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	eventProto "github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/clients/registry/mapping"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func setupReconciler(
	t *testing.T, targetRegions []string,
) (*eventprocessor.CryptoReconciler, *systems.FakeService, repo.Repo, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	r := sql.NewRepository(db)

	cfg := &config.Config{
		Database: dbCfg,
		Plugins:  testutils.SetupMockPlugins(testutils.KeyStorePlugin),
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

	ctlg, err := catalog.New(t.Context(), cfg)
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
		ctlg, clientsFactory,
	)
	assert.NoError(t, err)

	t.Cleanup(func() {
		eventProcessor.CloseAmqpClients(context.Background())
	})

	return eventProcessor, systemService, r, tenants[0]
}

func TestResolveKeyTasks(t *testing.T) {
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

	reconciler, _, r, tenant := setupReconciler(t, []string{
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

	allTargets := []string{connectedSystem.Region, disconnectedSystem.Region, keylessSystem.Region, systemlessTarget}

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
				expTargets: allTargets,
			},
			{
				name:       "KEY_DELETE task",
				taskType:   eventProto.TaskType_KEY_DELETE.String(),
				expTargets: allTargets,
			},
			{
				name:       "KEY_ROTATE task",
				taskType:   eventProto.TaskType_KEY_ROTATE.String(),
				expTargets: allTargets,
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

func TestResolveSystemTasks(t *testing.T) {
	// given
	region := "test-region"
	reconciler, _, r, tenant := setupReconciler(t, []string{region})
	resolveTaskFn := reconciler.ResolveTasks()

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	keyConfiguration := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	system := testutils.NewSystem(func(s *model.System) {
		s.Region = region
	})

	keyFrom := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfiguration.ID
		k.Provider = "TEST"
		k.NativeID = ptr.PointTo("key-from-native-id")
		k.CryptoAccessData = []byte(`{"test-region":{"keyX":"value1"}}`)
	})

	keyTo := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfiguration.ID
		k.Provider = "TEST"
		k.NativeID = ptr.PointTo("key-to-native-id")
		k.CryptoAccessData = []byte(`{"test-region":{"keyX":"value2"}}`)
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfiguration, system, keyFrom, keyTo)

	t.Run("should return correct task info for", func(t *testing.T) {
		tests := []struct {
			name     string
			taskType string
			data     func() []byte
		}{
			{
				name:     "SYSTEM_LINK task",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: system.ID.String(),
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
			},
			{
				name:     "SYSTEM_UNLINK task",
				taskType: eventProto.TaskType_SYSTEM_UNLINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID:  tenant,
						SystemID:  system.ID.String(),
						KeyIDFrom: keyFrom.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
			},
			{
				name:     "SYSTEM_SWITCH task",
				taskType: eventProto.TaskType_SYSTEM_SWITCH.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID:  tenant,
						SystemID:  system.ID.String(),
						KeyIDFrom: keyFrom.ID.String(),
						KeyIDTo:   keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := orbital.NewJob(tt.taskType, tt.data())

				// when
				result, err := resolveTaskFn(ctx, j, "")

				// then
				assert.NoError(t, err)
				assert.Empty(t, result.CanceledErrorMessage)
				assert.True(t, result.Done)
				if assert.Len(t, result.TaskInfos, 1) {
					ti := result.TaskInfos[0]
					assert.Equal(t, tt.taskType, ti.Type)
					assert.Equal(t, system.Region, ti.Target)

					var act eventProto.Data
					assert.NoError(t, proto.Unmarshal(ti.Data, &act))
					sa := act.GetSystemAction()
					assert.NotNil(t, sa)
					assert.Equal(t, system.Identifier, sa.GetSystemId())
					assert.Equal(t, system.Region, sa.GetSystemRegion())
					assert.Equal(t, tenant, sa.GetTenantId())

					switch tt.taskType {
					case eventProto.TaskType_SYSTEM_LINK.String():
						assert.Equal(t, keyTo.ID.String(), sa.GetKeyIdTo())
						assert.Empty(t, sa.GetKeyIdFrom())
					case eventProto.TaskType_SYSTEM_UNLINK.String():
						assert.Equal(t, keyFrom.ID.String(), sa.GetKeyIdFrom())
						assert.Empty(t, sa.GetKeyIdTo())
					case eventProto.TaskType_SYSTEM_SWITCH.String():
						assert.Equal(t, keyFrom.ID.String(), sa.GetKeyIdFrom())
						assert.Equal(t, keyTo.ID.String(), sa.GetKeyIdTo())
					}
				}
			})
		}
	})

	t.Run("should cancel task for", func(t *testing.T) {
		// system with region not configured
		sysNC := testutils.NewSystem(func(s *model.System) {
			s.Region = "not-configured"
		})
		assert.NoError(t, r.Create(ctx, sysNC))

		tests := []struct {
			name          string
			taskType      string
			data          func() []byte
			cancelMessage string
		}{
			{
				name:     "invalid JSON data",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					return []byte("{invalid-json}")
				},
				cancelMessage: "failed to unmarshal job data",
			},
			{
				name:     "missing tenant ID",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						SystemID: system.ID.String(),
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				cancelMessage: "record not found",
			},
			{
				name:     "missing system ID",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				cancelMessage: "invalid input syntax",
			},
			{
				name:     "system not found",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: uuid.NewString(),
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				cancelMessage: "record not found",
			},
			{
				name:     "target region not configured",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: sysNC.ID.String(),
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				cancelMessage: "target not configured for region",
			},
			{
				name:     "missing key ID for LINK",
				taskType: eventProto.TaskType_SYSTEM_LINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: system.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				cancelMessage: "failed to get key by ID",
			},
			{
				name:     "missing key ID for UNLINK",
				taskType: eventProto.TaskType_SYSTEM_UNLINK.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: system.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				cancelMessage: "failed to get key by ID",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := orbital.NewJob(tt.taskType, tt.data())

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

func TestConfirmJob(t *testing.T) {
	reconciler, _, r, tenant := setupReconciler(t, []string{"r1"})

	t.Run("key tasks are confirmed as done", func(t *testing.T) {
		keyJobTypes := []string{
			eventProto.TaskType_KEY_DELETE.String(),
			eventProto.TaskType_KEY_ROTATE.String(),
			eventProto.TaskType_KEY_DISABLE.String(),
			eventProto.TaskType_KEY_ENABLE.String(),
			eventProto.TaskType_KEY_DETACH.String(),
		}

		for _, tt := range keyJobTypes {
			t.Run(tt, func(t *testing.T) {
				data := eventprocessor.KeyActionJobData{
					TenantID: tenant,
					KeyID:    uuid.NewString(),
				}
				b, err := json.Marshal(data)
				assert.NoError(t, err)

				job := orbital.NewJob(tt, b)
				res, err := reconciler.ConfirmJob(t.Context(), job)
				assert.NoError(t, err)
				assert.True(t, res.Done)
				assert.False(t, res.IsCanceled)
			})
		}
	})

	t.Run("unsupported task type returns error", func(t *testing.T) {
		job := orbital.NewJob("UNKNOWN_TASK_TYPE", []byte("{}"))
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.Error(t, err)
		assert.False(t, res.Done)
	})

	t.Run("invalid JSON for system task returns error", func(t *testing.T) {
		job := orbital.NewJob(eventProto.TaskType_SYSTEM_LINK.String(), []byte("{invalid-json}"))
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.Error(t, err)
		assert.False(t, res.Done)
	})

	t.Run("missing system returns canceled with error", func(t *testing.T) {
		data := eventprocessor.SystemActionJobData{
			TenantID: tenant,
			SystemID: uuid.NewString(), // not in DB
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventProto.TaskType_SYSTEM_LINK.String(), b)
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.Error(t, err)
		assert.False(t, res.Done)
		assert.False(t, res.IsCanceled) // function returns error with Done:false for missing system
	})

	t.Run("system not in PROCESSING is canceled", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID: tenant,
			SystemID: sys.ID.String(),
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventProto.TaskType_SYSTEM_LINK.String(), b)
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.NoError(t, err)
		assert.True(t, res.IsCanceled)
		assert.Contains(t, res.CanceledErrorMessage, "system status is in")
	})

	t.Run("system in PROCESSING is confirmed as done", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID: tenant,
			SystemID: sys.ID.String(),
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventProto.TaskType_SYSTEM_LINK.String(), b)
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.NoError(t, err)
		assert.True(t, res.Done)
	})
}

func TestJobTermination(t *testing.T) {
	eventProcessor, systemService, r, tenant := setupReconciler(t, []string{})
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

	t.Run("Should call UnmapSystemFromTenant on decommission trigger", func(t *testing.T) {
		// Prepare entities
		sys := testutils.NewSystem(func(_ *model.System) {})
		keyCfg := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyCfg.ID
		})
		testutils.CreateTestEntities(ctx, t, r, sys, keyCfg, key)

		// Build unlink job data with decomission trigger
		decomJobData := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  sys.ID.String(),
			KeyIDFrom: key.ID.String(),
			Trigger:   constants.SystemActionDecommission,
		}
		b, err := json.Marshal(decomJobData)
		assert.NoError(t, err)

		// Terminate unlink with Done to invoke UnmapSystemFromTenant branch
		job := orbital.Job{
			ExternalID: uuid.NewString(),
			Type:       eventProto.TaskType_SYSTEM_UNLINK.String(),
			Data:       b,
			Status:     orbital.JobStatusDone,
		}

		err = eventProcessor.JobTerminationFunc(t.Context(), job)
		assert.NoError(t, err)

		// Assert system is disconnected after unlink
		sysAfter := &model.System{ID: sys.ID}
		_, err = r.First(ctx, sysAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusDISCONNECTED, sysAfter.Status)
	})
}

func TestWithOptions(t *testing.T) {
	t.Run("WithMaxReconcileCount", func(t *testing.T) {
		var m orbital.Manager
		opt := eventprocessor.WithMaxReconcileCount(42)
		opt(&m)
		assert.Equal(t, uint64(42), m.Config.MaxReconcileCount)
	})

	t.Run("WithConfirmJobAfter", func(t *testing.T) {
		var m orbital.Manager
		d := 7 * time.Second
		opt := eventprocessor.WithConfirmJobAfter(d)
		opt(&m)
		assert.Equal(t, d, m.Config.ConfirmJobAfter)
	})

	t.Run("WithExecInterval", func(t *testing.T) {
		var m orbital.Manager
		d := 1500 * time.Millisecond
		opt := eventprocessor.WithExecInterval(d)
		opt(&m)
		assert.Equal(t, d, m.Config.ReconcileWorkerConfig.ExecInterval)
		assert.Equal(t, d, m.Config.CreateTasksWorkerConfig.ExecInterval)
		assert.Equal(t, d, m.Config.ConfirmJobWorkerConfig.ExecInterval)
		assert.Equal(t, d, m.Config.NotifyWorkerConfig.ExecInterval)
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
