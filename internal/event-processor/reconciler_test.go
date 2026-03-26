package eventprocessor_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

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
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

type TestInstance struct {
	repo          repo.Repo
	tenant        string
	fakeService   *systems.FakeService
	reconciler    *eventprocessor.CryptoReconciler
	traceRecorder *tracetest.SpanRecorder
}

func setupTestInstance(
	t *testing.T, targetRegions []string,
) TestInstance {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	r := sql.NewRepository(db)

	ps, psCfg := testutils.NewTestPlugins(testplugins.NewKeystoreOperator())

	cfg := &config.Config{
		Database: dbCfg,
		Plugins:  psCfg,
		Landscape: config.Landscape{
			Region: uuid.NewString(),
		},
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name: "event-processor",
			},
			Telemetry: commoncfg.Telemetry{
				Traces: commoncfg.Trace{
					Enabled: true,
				},
			},
		},
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

	svcRegistry, err := cmkpluginregistry.New(t.Context(), cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
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

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	eventProcessor, err := eventprocessor.NewCryptoReconciler(
		t.Context(), cfg, r,
		svcRegistry, clientsFactory,
	)
	assert.NoError(t, err)

	eventProcessor.DisableAuditLog()

	t.Cleanup(func() {
		eventProcessor.CloseAmqpClients(context.Background())
		otel.SetTracerProvider(original)
		_ = tp.Shutdown(t.Context())
	})

	return TestInstance{
		repo:          r,
		tenant:        tenants[0],
		fakeService:   systemService,
		reconciler:    eventProcessor,
		traceRecorder: recorder,
	}
}

func TestGetHandlerByJobType(t *testing.T) {
	instance := setupTestInstance(t, []string{"region1"})
	reconciler := instance.reconciler

	t.Run("returns handler for supported job type", func(t *testing.T) {
		handler, err := reconciler.GetHandlerByJobType(string(eventprocessor.JobTypeSystemLink))
		assert.NoError(t, err)
		assert.NotNil(t, handler)
	})

	t.Run("returns error for unsupported job type", func(t *testing.T) {
		handler, err := reconciler.GetHandlerByJobType("nonexistent-job-type")
		assert.Error(t, err)
		assert.Nil(t, handler)
	})

	t.Run("returns error for empty job type", func(t *testing.T) {
		handler, err := reconciler.GetHandlerByJobType("")
		assert.Error(t, err)
		assert.Nil(t, handler)
	})
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

	instance := setupTestInstance(t, []string{
		connectedSystem.Region,
		disconnectedSystem.Region,
		keylessSystem.Region,
		systemlessTarget,
	})
	reconciler := instance.reconciler
	r := instance.repo
	tenant := instance.tenant

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	for _, sys := range []*model.System{connectedSystem, disconnectedSystem, targetlessSystem, keylessSystem} {
		err := r.Create(cmkcontext.CreateTenantContext(t.Context(), tenant), sys)
		assert.NoError(t, err)
	}

	allTargets := []string{connectedSystem.Region, disconnectedSystem.Region, keylessSystem.Region, systemlessTarget}

	t.Run("should resolve targets for", func(t *testing.T) {
		tests := []struct {
			name       string
			jobType    string
			taskType   string
			expTargets []string
		}{
			{
				name:       "KEY_ENABLE task",
				jobType:    eventprocessor.JobTypeKeyEnable.String(),
				taskType:   eventProto.TaskType_KEY_ENABLE.String(),
				expTargets: []string{connectedSystem.Region},
			},
			{
				name:       "KEY_DISABLE task",
				jobType:    eventprocessor.JobTypeKeyDisable.String(),
				taskType:   eventProto.TaskType_KEY_DISABLE.String(),
				expTargets: []string{connectedSystem.Region},
			},
			{
				name:       "KEY_DETACH task",
				jobType:    eventprocessor.JobTypeKeyDetach.String(),
				taskType:   eventProto.TaskType_KEY_DETACH.String(),
				expTargets: allTargets,
			},
			{
				name:       "KEY_DELETE task",
				jobType:    eventprocessor.JobTypeKeyDelete.String(),
				taskType:   eventProto.TaskType_KEY_DELETE.String(),
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

				j := orbital.NewJob(tt.jobType, dataBytes)
				handler, err := reconciler.GetHandlerByJobType(tt.jobType)
				assert.NoError(t, err)

				// when
				tasks, err := handler.ResolveTasks(ctx, j)

				// then
				assert.NoError(t, err)
				actTargets := make([]string, 0, len(tasks))
				for _, ti := range tasks {
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
			name         string
			data         func() []byte
			errorMessage string
		}{
			{
				name: "invalid JSON data",
				data: func() []byte {
					return []byte("{invalid-json}")
				},
				errorMessage: "failed to unmarshal job data",
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
				errorMessage: "record not found",
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
				errorMessage: "failed to get key by ID",
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
				errorMessage: "no connected regions found for key",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := orbital.NewJob(eventprocessor.JobTypeKeyEnable.String(), tt.data())
				handler, err := reconciler.GetHandlerByJobType(eventprocessor.JobTypeKeyEnable.String())
				assert.NoError(t, err)

				// when
				result, err := handler.ResolveTasks(ctx, j)

				// then
				assert.Error(t, err)
				assert.Empty(t, result)
				assert.Contains(t, err.Error(), tt.errorMessage)
			})
		}
	})
}

func TestResolveSystemTasks(t *testing.T) {
	// given
	region := "test-region"
	instance := setupTestInstance(t, []string{region})
	reconciler := instance.reconciler
	r := instance.repo
	tenant := instance.tenant

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
			jobType  string
			taskType string
			data     func() []byte
		}{
			{
				name:     "SYSTEM_LINK task",
				jobType:  eventprocessor.JobTypeSystemLink.String(),
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
				jobType:  eventprocessor.JobTypeSystemUnlink.String(),
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
				jobType:  eventprocessor.JobTypeSystemSwitch.String(),
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
			{
				name:     "SYSTEM_KEY_ROTATE task",
				jobType:  eventprocessor.JobTypeSystemKeyRotate.String(),
				taskType: eventProto.TaskType_SYSTEM_KEY_ROTATE.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID:  tenant,
						SystemID:  system.ID.String(),
						KeyIDTo:   keyTo.ID.String(),
						KeyIDFrom: keyTo.ID.String(), // Same key, different versions
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := orbital.NewJob(tt.jobType, tt.data())
				handler, err := reconciler.GetHandlerByJobType(tt.jobType)
				assert.NoError(t, err)

				// when
				tasks, err := handler.ResolveTasks(ctx, j)

				// then
				assert.NoError(t, err)
				if assert.Len(t, tasks, 1) {
					ti := tasks[0]
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
					case eventProto.TaskType_SYSTEM_KEY_ROTATE.String():
						assert.Equal(t, keyTo.ID.String(), sa.GetKeyIdTo())
						assert.Equal(t, keyTo.ID.String(), sa.GetKeyIdFrom()) // Same key
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
			name         string
			jobType      string
			data         func() []byte
			errorMessage string
		}{
			{
				name:    "invalid JSON data",
				jobType: eventprocessor.JobTypeSystemLink.String(),
				data: func() []byte {
					return []byte("{invalid-json}")
				},
				errorMessage: "failed to unmarshal job data",
			},
			{
				name:    "missing tenant ID",
				jobType: eventprocessor.JobTypeSystemLink.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						SystemID: system.ID.String(),
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				errorMessage: "record not found",
			},
			{
				name:    "missing system ID",
				jobType: eventprocessor.JobTypeSystemLink.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						KeyIDTo:  keyTo.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				errorMessage: "invalid input syntax",
			},
			{
				name:    "system not found",
				jobType: eventprocessor.JobTypeSystemLink.String(),
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
				errorMessage: "record not found",
			},
			{
				name:    "target region not configured",
				jobType: eventprocessor.JobTypeSystemLink.String(),
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
				errorMessage: "target not configured for region",
			},
			{
				name:    "missing key ID for LINK",
				jobType: eventprocessor.JobTypeSystemLink.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: system.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				errorMessage: "failed to get key by ID",
			},
			{
				name:    "missing key ID for UNLINK",
				jobType: eventprocessor.JobTypeSystemUnlink.String(),
				data: func() []byte {
					d := eventprocessor.SystemActionJobData{
						TenantID: tenant,
						SystemID: system.ID.String(),
					}
					b, err := json.Marshal(d)
					assert.NoError(t, err)
					return b
				},
				errorMessage: "failed to get key by ID",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := orbital.NewJob(tt.jobType, tt.data())

				// when
				handler, err := reconciler.GetHandlerByJobType(tt.jobType)
				assert.NoError(t, err)

				// when
				tasks, err := handler.ResolveTasks(ctx, j)

				// then
				assert.Error(t, err)
				assert.Empty(t, tasks)
				assert.Contains(t, err.Error(), tt.errorMessage)
			})
		}
	})

	t.Run("should add trace with correct attributes", func(t *testing.T) {
		jobType := eventprocessor.JobTypeSystemLink.String()
		j := orbital.NewJob(jobType, []byte("{}"))

		instance.traceRecorder.Reset()
		result, err := instance.reconciler.ResolveTasks()(t.Context(), j, "")
		assert.NoError(t, err) // No error here, but the job should be canceled due to invalid JSON and trace should still record error
		assert.IsType(t, orbital.CancelTaskResolver(""), result)

		spans := instance.traceRecorder.Ended()
		assert.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, "ResolveTasks:SYSTEM_LINK", span.Name())
		assert.Equal(t, "job.id", string(span.Attributes()[0].Key))
		assert.Equal(t, j.ID.String(), span.Attributes()[0].Value.AsString())
		assert.Equal(t, "job.type", string(span.Attributes()[1].Key))
		assert.Equal(t, jobType, span.Attributes()[1].Value.AsString())
	})
}

func TestConfirmJob(t *testing.T) {
	instance := setupTestInstance(t, []string{"r1"})
	reconciler := instance.reconciler
	r := instance.repo
	tenant := instance.tenant

	t.Run("key tasks are confirmed as done", func(t *testing.T) {
		keyJobTypes := []string{
			eventprocessor.JobTypeKeyDelete.String(),
			eventprocessor.JobTypeKeyDisable.String(),
			eventprocessor.JobTypeKeyEnable.String(),
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
				handler, err := reconciler.GetHandlerByJobType(tt)
				assert.NoError(t, err)

				res, err := handler.HandleJobConfirm(t.Context(), job)
				assert.NoError(t, err)
				assert.IsType(t, orbital.CompleteJobConfirmer(), res)
			})
		}
	})

	t.Run("key detach task is confirmed as done", func(t *testing.T) {
		kc := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {})
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = kc.ID
			k.State = string(cmkapi.KeyStateDETACHING)
		})
		ctx := testutils.CreateCtxWithTenant(tenant)
		testutils.CreateTestEntities(ctx, t, r, kc, key)

		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    key.ID.String(),
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeKeyDetach.String(), b)
		handler, err := reconciler.GetHandlerByJobType(eventprocessor.JobTypeKeyDetach.String())
		assert.NoError(t, err)

		res, err := handler.HandleJobConfirm(t.Context(), job)
		assert.NoError(t, err)
		assert.IsType(t, orbital.CompleteJobConfirmer(), res)
	})

	t.Run("key detach task with missing key is canceled", func(t *testing.T) {
		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    uuid.NewString(), // not in DB
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeKeyDetach.String(), b)
		handler, err := reconciler.GetHandlerByJobType(eventprocessor.JobTypeKeyDetach.String())
		assert.NoError(t, err)

		res, err := handler.HandleJobConfirm(t.Context(), job)
		assert.NoError(t, err)
		assert.IsType(t, orbital.CancelJobConfirmer(""), res)
	})

	t.Run("key detach task with key not in DETACHING state is canceled", func(t *testing.T) {
		kc := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {})
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = kc.ID
		})
		ctx := testutils.CreateCtxWithTenant(tenant)
		testutils.CreateTestEntities(ctx, t, r, kc, key)

		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    key.ID.String(),
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeKeyDetach.String(), b)
		handler, err := reconciler.GetHandlerByJobType(eventprocessor.JobTypeKeyDetach.String())
		assert.NoError(t, err)

		res, err := handler.HandleJobConfirm(t.Context(), job)
		assert.NoError(t, err)
		assert.IsType(t, orbital.CancelJobConfirmer(""), res)
	})

	t.Run("unsupported task type returns error", func(t *testing.T) {
		job := orbital.NewJob("UNKNOWN_TASK_TYPE", []byte("{}"))
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported job type")
		assert.IsType(t, orbital.CancelJobConfirmer(""), res)
	})

	t.Run("invalid JSON for system task returns error", func(t *testing.T) {
		job := orbital.NewJob(eventprocessor.JobTypeSystemLink.String(), []byte("{invalid-json}"))
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal job data")
		assert.IsType(t, orbital.CancelJobConfirmer(""), res)
	})

	t.Run("missing system returns canceled with error", func(t *testing.T) {
		data := eventprocessor.SystemActionJobData{
			TenantID: tenant,
			SystemID: uuid.NewString(), // not in DB
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeSystemLink.String(), b)
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.NoError(t, err)
		assert.IsType(t, orbital.CancelJobConfirmer(""), res)
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

		job := orbital.NewJob(eventprocessor.JobTypeSystemLink.String(), b)
		res, err := reconciler.ConfirmJob(t.Context(), job)
		assert.NoError(t, err)
		assert.IsType(t, orbital.CancelJobConfirmer(""), res)
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

		systemJobTypes := []string{
			eventprocessor.JobTypeSystemLink.String(),
			eventprocessor.JobTypeSystemUnlink.String(),
			eventprocessor.JobTypeSystemSwitch.String(),
			eventprocessor.JobTypeSystemSwitchNewPK.String(),
		}

		for _, jobType := range systemJobTypes {
			t.Run(jobType, func(t *testing.T) {
				job := orbital.NewJob(jobType, b)
				res, err := reconciler.ConfirmJob(t.Context(), job)
				assert.NoError(t, err)
				assert.IsType(t, orbital.CompleteJobConfirmer(), res)
			})
		}
	})

	t.Run("should add trace with correct attributes", func(t *testing.T) {
		jobType := eventprocessor.JobTypeSystemLink.String()
		j := orbital.NewJob(jobType, []byte("{}"))

		instance.traceRecorder.Reset()
		_, err := instance.reconciler.ConfirmJob(t.Context(), j)
		assert.Error(t, err)

		spans := instance.traceRecorder.Ended()
		assert.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, "ConfirmJob:SYSTEM_LINK", span.Name())
		assert.Equal(t, "job.id", string(span.Attributes()[0].Key))
		assert.Equal(t, j.ID.String(), span.Attributes()[0].Value.AsString())
		assert.Equal(t, "job.type", string(span.Attributes()[1].Key))
		assert.Equal(t, jobType, span.Attributes()[1].Value.AsString())
	})
}

func TestJobTermination(t *testing.T) {
	instance := setupTestInstance(t, []string{})
	eventProcessor := instance.reconciler
	r := instance.repo
	tenant := instance.tenant
	systemService := instance.fakeService

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
			Type:       eventprocessor.JobTypeSystemLink.String(),
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
			Type:       eventprocessor.JobTypeSystemUnlink.String(),
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
			Type:       eventprocessor.JobTypeSystemLink.String(),
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
			Type:       eventprocessor.JobTypeSystemUnlink.String(),
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
			Type:       eventprocessor.JobTypeSystemLink.String(),
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
			Type:       eventprocessor.JobTypeSystemLink.String(),
			Data:       dataBytes,
			Status:     orbital.JobStatusProcessing,
		}
		err := r.Create(ctx, event)
		assert.NoError(t, err)

		terminateNewJob(t, eventProcessor, event, false)

		_, err = r.First(ctx, event, *repo.NewQuery())
		assert.NoError(t, err)
	})

	t.Run("System status on canceled job termination", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  sys.ID.String(),
			KeyIDFrom: key.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		err = eventProcessor.JobCanceledFunc(t.Context(), orbital.Job{
			ExternalID: item,
			Type:       eventprocessor.JobTypeSystemLink.String(),
			Data:       dataBytes,
		})
		assert.NoError(t, err)

		sysAfter := &model.System{
			ID: sys.ID,
		}
		_, err = r.First(ctx, sysAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusFAILED, sysAfter.Status)
	})

	t.Run("System switch to new key on successful SYSTEM_SWITCH job termination", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		keyConfig2 := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		key2 := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig2.ID
		})
		testutils.CreateTestEntities(ctx, t, r, keyConfig2, key2)
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  sys.ID.String(),
			KeyIDFrom: key.ID.String(),
			KeyIDTo:   key2.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventprocessor.JobTypeSystemSwitch.String(),
			Data:       dataBytes,
		}, true)

		sysAfter := &model.System{
			ID: sys.ID,
		}
		_, err = r.First(ctx, sysAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, keyConfig2.ID, *sysAfter.KeyConfigurationID)
	})

	t.Run("System status on success SYSTEM_UNLINK_DECOMMISSION job termination", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  sys.ID.String(),
			KeyIDFrom: key.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventprocessor.JobTypeSystemUnlinkDecommission.String(),
			Data:       dataBytes,
		}, true)

		sysAfter := &model.System{
			ID: sys.ID,
		}
		_, err = r.First(ctx, sysAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusDISCONNECTED, sysAfter.Status)
	})

	t.Run("System status on failed SYSTEM_UNLINK_DECOMMISSION job termination", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  sys.ID.String(),
			KeyIDFrom: key.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventprocessor.JobTypeSystemUnlinkDecommission.String(),
			Data:       dataBytes,
		}, false)

		sysAfter := &model.System{
			ID: sys.ID,
		}
		_, err = r.First(ctx, sysAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusDISCONNECTED, sysAfter.Status)
	})

	t.Run("System status on canceled SYSTEM_UNLINK_DECOMMISSION job termination", func(t *testing.T) {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		assert.NoError(t, r.Create(testutils.CreateCtxWithTenant(tenant), sys))

		data := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  sys.ID.String(),
			KeyIDFrom: key.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		err = eventProcessor.JobCanceledFunc(t.Context(), orbital.Job{
			ExternalID: item,
			Type:       eventprocessor.JobTypeSystemUnlinkDecommission.String(),
			Data:       dataBytes,
		})
		assert.NoError(t, err)

		sysAfter := &model.System{
			ID: sys.ID,
		}
		_, err = r.First(ctx, sysAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusFAILED, sysAfter.Status)
	})

	t.Run("Should update key state to DETACHED on successful key detach job termination", func(t *testing.T) {
		keyToDetach := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})
		assert.NoError(t, r.Create(ctx, keyToDetach))

		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    keyToDetach.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventprocessor.JobTypeKeyDetach.String(),
			Data:       dataBytes,
		}, true)

		keyAfter := &model.Key{
			ID: keyToDetach.ID,
		}
		_, err = r.First(ctx, keyAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateDETACHED), keyAfter.State)
	})

	t.Run("Should update key state to DETACHED on failed key detach job termination", func(t *testing.T) {
		keyToDetach := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})
		assert.NoError(t, r.Create(ctx, keyToDetach))

		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    keyToDetach.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventprocessor.JobTypeKeyDetach.String(),
			Data:       dataBytes,
		}, false)

		keyAfter := &model.Key{
			ID: keyToDetach.ID,
		}
		_, err = r.First(ctx, keyAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateDETACHED), keyAfter.State)
	})

	t.Run("Should update key state to UNKNOWN on canceled key detach job termination", func(t *testing.T) {
		keyToDetach := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})
		assert.NoError(t, r.Create(ctx, keyToDetach))

		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    keyToDetach.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		item := uuid.NewString()
		err = eventProcessor.JobCanceledFunc(t.Context(), orbital.Job{
			ExternalID: item,
			Type:       eventprocessor.JobTypeKeyDetach.String(),
			Data:       dataBytes,
		})
		assert.NoError(t, err)

		keyAfter := &model.Key{
			ID: keyToDetach.ID,
		}
		_, err = r.First(ctx, keyAfter, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, string(cmkapi.KeyStateUNKNOWN), keyAfter.State)
	})

	t.Run("should add trace with correct attributes for JobDone", func(t *testing.T) {
		jobType := eventprocessor.JobTypeSystemLink.String()
		j := orbital.NewJob(jobType, []byte("{}"))

		instance.traceRecorder.Reset()
		err := instance.reconciler.JobDoneFunc(t.Context(), j)
		assert.Error(t, err)

		spans := instance.traceRecorder.Ended()
		assert.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, "JobDone:SYSTEM_LINK", span.Name())
		assert.Equal(t, "job.id", string(span.Attributes()[0].Key))
		assert.Equal(t, j.ID.String(), span.Attributes()[0].Value.AsString())
		assert.Equal(t, "job.type", string(span.Attributes()[1].Key))
		assert.Equal(t, jobType, span.Attributes()[1].Value.AsString())
	})

	t.Run("should add trace with correct attributes for JobFailed", func(t *testing.T) {
		jobType := eventprocessor.JobTypeSystemLink.String()
		j := orbital.NewJob(jobType, []byte("{}"))

		instance.traceRecorder.Reset()
		err := instance.reconciler.JobFailedFunc(t.Context(), j)
		assert.Error(t, err)

		spans := instance.traceRecorder.Ended()
		assert.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, "JobFailed:SYSTEM_LINK", span.Name())
		assert.Equal(t, "job.id", string(span.Attributes()[0].Key))
		assert.Equal(t, j.ID.String(), span.Attributes()[0].Value.AsString())
		assert.Equal(t, "job.type", string(span.Attributes()[1].Key))
		assert.Equal(t, jobType, span.Attributes()[1].Value.AsString())
	})

	t.Run("should add trace with correct attributes for JobCanceled", func(t *testing.T) {
		jobType := eventprocessor.JobTypeSystemLink.String()
		j := orbital.NewJob(jobType, []byte("{}"))

		instance.traceRecorder.Reset()
		err := instance.reconciler.JobCanceledFunc(t.Context(), j)
		assert.Error(t, err)

		spans := instance.traceRecorder.Ended()
		assert.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, "JobCanceled:SYSTEM_LINK", span.Name())
		assert.Equal(t, "job.id", string(span.Attributes()[0].Key))
		assert.Equal(t, j.ID.String(), span.Attributes()[0].Value.AsString())
		assert.Equal(t, "job.type", string(span.Attributes()[1].Key))
		assert.Equal(t, jobType, span.Attributes()[1].Value.AsString())
	})
}

func TestSystemKeyRotateJobHandler(t *testing.T) {
	instance := setupTestInstance(t, []string{})
	r := instance.repo
	tenant := instance.tenant

	ctx := testutils.CreateCtxWithTenant(tenant)

	system := testutils.NewSystem(func(_ *model.System) {})
	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	testutils.CreateTestEntities(ctx, t, r, system, keyConfig, key)

	// Helper to create standard job data and marshal it
	createJobData := func() []byte {
		jobData := eventprocessor.SystemActionJobData{
			TenantID:  tenant,
			SystemID:  system.ID.String(),
			KeyIDTo:   key.ID.String(),
			KeyIDFrom: key.ID.String(),
		}
		dataBytes, err := json.Marshal(jobData)
		assert.NoError(t, err)
		return dataBytes
	}

	// Helper to create event and return its ID
	createEvent := func(dataBytes []byte, previousStatus string) string {
		eventID := uuid.NewString()
		event := &model.Event{
			Identifier:         eventID,
			Type:               eventprocessor.JobTypeSystemKeyRotate.String(),
			Data:               dataBytes,
			PreviousItemStatus: previousStatus,
		}
		err := r.Set(ctx, event)
		assert.NoError(t, err)
		return eventID
	}

	// Helper to get handler
	getHandler := func() eventprocessor.JobHandler {
		handler, err := instance.reconciler.GetHandlerByJobType(eventprocessor.JobTypeSystemKeyRotate.String())
		assert.NoError(t, err)
		return handler
	}

	t.Run("HandleJobDoneEvent should clean up event", func(t *testing.T) {
		dataBytes := createJobData()
		eventID := createEvent(dataBytes, string(cmkapi.SystemStatusCONNECTED))

		job := orbital.NewJob(eventprocessor.JobTypeSystemKeyRotate.String(), dataBytes).WithExternalID(eventID)

		err := getHandler().HandleJobDoneEvent(ctx, job)
		assert.NoError(t, err)

		// Verify event was cleaned up (deleted from database)
		found, err := r.First(ctx, &model.Event{Identifier: eventID}, *repo.NewQuery())
		if err != nil && !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("unexpected error checking event: %v", err)
		}
		assert.False(t, found)
	})

	t.Run("HandleJobFailedEvent with version mismatch should not set system to FAILED", func(t *testing.T) {
		dataBytes := createJobData()

		system.Status = cmkapi.SystemStatusCONNECTED
		_, err := r.Patch(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)

		eventID := uuid.NewString()
		event := &model.Event{
			Identifier: eventID,
			Type:       eventprocessor.JobTypeSystemKeyRotate.String(),
			Data:       dataBytes,
		}
		err = r.Set(ctx, event)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeSystemKeyRotate.String(), dataBytes).
			WithExternalID(eventID)
		job.ErrorMessage = "KEY_VERSION_MISMATCH:Version mismatch detected"

		err = getHandler().HandleJobFailedEvent(ctx, job)
		assert.NoError(t, err)

		// Verify system status is still CONNECTED (not FAILED)
		_, err = r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusCONNECTED, system.Status)

		// Verify event was cleaned up (version mismatch doesn't need retry)
		found, err := r.First(ctx, &model.Event{Identifier: eventID}, *repo.NewQuery())
		if err != nil && !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("unexpected error checking event: %v", err)
		}
		assert.False(t, found)
	})

	t.Run("HandleJobFailedEvent with other error should set system to FAILED", func(t *testing.T) {
		dataBytes := createJobData()

		system.Status = cmkapi.SystemStatusPROCESSING
		_, err := r.Patch(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)

		eventID := createEvent(dataBytes, string(cmkapi.SystemStatusCONNECTED))

		job := orbital.NewJob(eventprocessor.JobTypeSystemKeyRotate.String(), dataBytes).
			WithExternalID(eventID)
		job.ErrorMessage = "SOME_OTHER_ERROR:Something went wrong"

		err = getHandler().HandleJobFailedEvent(ctx, job)
		assert.NoError(t, err)

		// Verify system status is FAILED
		_, err = r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusFAILED, system.Status)

		// Verify error was stored in event
		event := &model.Event{Identifier: eventID}
		found, err := r.First(ctx, event, *repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "SOME_OTHER_ERROR", event.ErrorCode)
		assert.Equal(t, "Something went wrong", event.ErrorMessage)
	})

	t.Run("HandleJobCanceledEvent should not change system status", func(t *testing.T) {
		dataBytes := createJobData()

		system.Status = cmkapi.SystemStatusPROCESSING
		_, err := r.Patch(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)

		eventID := createEvent(dataBytes, string(cmkapi.SystemStatusCONNECTED))

		job := orbital.NewJob(eventprocessor.JobTypeSystemKeyRotate.String(), dataBytes).
			WithExternalID(eventID)
		job.ErrorMessage = "Job canceled by user"

		err = getHandler().HandleJobCanceledEvent(ctx, job)
		assert.NoError(t, err)

		// Verify system status is unchanged (still PROCESSING)
		_, err = r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusPROCESSING, system.Status)

		// Verify event was cleaned up (canceled job won't be retried)
		found, err := r.First(ctx, &model.Event{Identifier: eventID}, *repo.NewQuery())
		if err != nil && !errors.Is(err, repo.ErrNotFound) {
			t.Fatalf("unexpected error checking event: %v", err)
		}
		assert.False(t, found)
	})
}

func TestWithOptions(t *testing.T) {
	t.Run("WithMaxReconcileCount", func(t *testing.T) {
		var m orbital.Manager
		opt := eventprocessor.WithMaxPendingReconciles(42)
		opt(&m)
		assert.Equal(t, uint64(42), m.Config.MaxPendingReconciles)
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

	var err error
	if jobDone {
		job.Status = orbital.JobStatusDone
		err = eventProcessor.JobDoneFunc(t.Context(), job)
	} else {
		job.Status = orbital.JobStatusFailed
		err = eventProcessor.JobFailedFunc(t.Context(), job)
	}

	// Ignored as this test is not testing the system/key update capabilities
	if err != nil {
		t.Logf("Job termination returned error: %v", err)
	}
}
