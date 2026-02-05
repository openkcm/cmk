package operator_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/respondertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry/tenants"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/operator"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	mockClient "github.com/openkcm/cmk/internal/testutils/clients"
	"github.com/openkcm/cmk/internal/testutils/clients/registry"
	sessionmanager "github.com/openkcm/cmk/internal/testutils/clients/session-manager"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
	tmdb "github.com/openkcm/cmk/utils/base62"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	RegionUSWest1 = "us-west-1"
	AMQPURL       = "amqp://guest:guest@localhost:5672"
	AMQPTarget    = "cmk.global.tenants"
	AMQPSource    = "cmk.emea.tenants"
)

type MockTenantManager struct {
	mockOffboardTenant func(ctx context.Context) (manager.OffboardingResult, error)
	mockDeleteTenant   func(ctx context.Context) error
}

func (m *MockTenantManager) GetTenant(_ context.Context) (*model.Tenant, error) {
	return &model.Tenant{}, nil
}

func (m *MockTenantManager) ListTenantInfo(_ context.Context, _ *string, _ int, _ int) ([]*model.Tenant, int, error) {
	return nil, 0, nil
}

func (m *MockTenantManager) CreateTenant(_ context.Context, _ *model.Tenant) error {
	return nil
}

func (m *MockTenantManager) OffboardTenant(ctx context.Context) (manager.OffboardingResult, error) {
	return m.mockOffboardTenant(ctx)
}

func (m *MockTenantManager) DeleteTenant(ctx context.Context) error {
	return m.mockDeleteTenant(ctx)
}

func createManagers(
	t *testing.T,
	dbCon *multitenancy.DB,
	cfg *config.Config,
) (*manager.TenantManager, *manager.GroupManager) {
	t.Helper()

	r := sql.NewRepository(dbCon)
	ctx := t.Context()

	ctlg, err := catalog.New(ctx, cfg)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(ctx, cfg)

	f, err := clients.NewFactory(config.Services{})
	assert.NoError(t, err)

	cm := manager.NewCertificateManager(ctx, r, ctlg, &cfg.Certificates)
	um := manager.NewUserManager(r, cmkAuditor)
	tagm := manager.NewTagManager(r)
	kcm := manager.NewKeyConfigManager(r, cm, um, tagm, cmkAuditor, cfg)

	sys := manager.NewSystemManager(
		ctx,
		r,
		f,
		nil,
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
		nil,
		cmkAuditor,
	)

	migrator, err := db.NewMigrator(r, cfg)
	assert.NoError(t, err)

	return manager.NewTenantManager(r, sys, km, um, cmkAuditor, migrator), manager.NewGroupManager(r, ctlg, um)
}

func TestNewTenantOperator(t *testing.T) {
	amqpClient := &amqp.Client{}
	dbConn := &multitenancy.DB{}
	fts := tenants.NewFakeTenantService()

	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: testutils.TestDB,
	}

	_, grpcClient := testutils.NewGRPCSuite(
		t,
		func(s *grpc.Server) {
			tenantgrpc.RegisterServiceServer(s, fts)
		},
	)

	operatorTarget := orbital.OperatorTarget{
		Client: amqpClient,
	}

	clientFactory := mockClient.NewMockFactory(
		registry.NewMockService(nil, tenantgrpc.NewServiceClient(grpcClient), mappingv1.NewServiceClient(grpcClient)),
		sessionmanager.NewMockService(sessionmanager.NewFakeSessionManagerClient()),
	)

	tenantManager, groupManager := createManagers(t, dbConn, cfg)

	t.Run(
		"nil db", func(t *testing.T) {
			op, err := operator.NewTenantOperator(nil, operatorTarget, clientFactory, tenantManager, groupManager)
			assert.Nil(t, op)
			assert.Error(t, err)
		},
	)

	t.Run(
		"nil amqp", func(t *testing.T) {
			target := orbital.OperatorTarget{}
			op, err := operator.NewTenantOperator(dbConn, target, clientFactory, tenantManager, groupManager)
			assert.Nil(t, op)
			assert.Error(t, err)
		},
	)

	t.Run(
		"nil factory client", func(t *testing.T) {
			op, err := operator.NewTenantOperator(dbConn, operatorTarget, nil, tenantManager, groupManager)
			assert.Nil(t, op)
			assert.Error(t, err)
		},
	)

	t.Run(
		"valid operator", func(t *testing.T) {
			op, err := operator.NewTenantOperator(dbConn, operatorTarget, clientFactory, tenantManager, groupManager)
			require.NoError(t, err)
			assert.NotNil(t, op)
		},
	)
}

func TestRunOperator(t *testing.T) {
	testConfig := newTestOperator(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)

	go func(ctx context.Context) {
		done <- testConfig.TenantOperator.RunOperator(ctx)
	}(ctx)

	// Cancel the context to stop the operator
	cancel()

	// Wait for the operator to finish and check for errors
	select {
	case err := <-done:
		require.NoError(t, err) // The operator should return a nil error upon graceful shutdown.
	case <-time.After(5 * time.Second):
		assert.Fail(t, "RunOperator did not stop within the expected time after context cancellation")
	}
}

func TestHandleCreateTenant(t *testing.T) {
	// Initialize TenantOperator
	ctx := t.Context()
	testConfig := newTestOperator(t)

	validTenantID := uuid.NewString()
	validData, err := createValidTenantData(validTenantID, RegionUSWest1, "ValidTenant")
	require.NoError(t, err)

	tests := []struct {
		name       string
		data       []byte
		wantResult orbital.Result
		wantState  string
		wantErr    bool
		setup      func()
		checkDB    bool
		region     string
	}{
		{
			name:       "valid tenant creation - first probe",
			data:       validData,
			wantResult: orbital.ResultProcessing,
			wantState:  operator.WorkingStateTenantCreating,
			wantErr:    false,
			setup:      func() {},
			checkDB:    false,
			region:     RegionUSWest1,
		},
		{
			name:       "valid tenant creation - second probe (idempotent)",
			data:       validData,
			wantResult: orbital.ResultDone,
			wantState:  operator.WorkingStateTenantCreatedSuccessfully,
			wantErr:    false,
			setup: func() {
				// Create tenant first to simulate second probe
				req := buildRequest(uuid.New(), tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), validData)
				_, err = testConfig.TenantOperator.HandleCreateTenant(ctx, req)
				require.NoError(t, err)
			},
			checkDB: true,
			region:  RegionUSWest1,
		},
		{
			name:       "sending groups to registry fails",
			data:       validData,
			wantResult: orbital.ResultProcessing,
			wantState:  operator.WorkingStateSendingGroupsFailed,
			wantErr:    false,
			setup: func() {
				// First create the tenant schema and groups
				req := buildRequest(uuid.New(), tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), validData)
				_, err = testConfig.TenantOperator.HandleCreateTenant(ctx, req)
				require.NoError(t, err)

				// Configure the fake service to fail on the second call
				testConfig.FakeTenantService.SetTenantUserGroupsError = operator.ErrSendingGroupsFailed
			},
			checkDB: true,
			region:  RegionUSWest1,
		},
		{
			name:       "invalid proto data",
			data:       []byte("invalid-proto"),
			wantResult: orbital.ResultFailed,
			wantState:  operator.WorkingStateUnmarshallingFailed,
			wantErr:    true,
			setup:      func() {},
			checkDB:    false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				tt.setup()

				req := buildRequest(uuid.New(), tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), tt.data)
				resp, err := testConfig.TenantOperator.HandleCreateTenant(ctx, req)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				assert.Equal(t, tt.wantResult, resp.Result)

				if tt.wantState != "" {
					assert.Contains(t, string(resp.WorkingState), tt.wantState)
				}

				if tt.checkDB {
					schemaName, _ := tmdb.EncodeSchemaNameBase62(validTenantID)
					integrationutils.TenantExists(t, testConfig.DB, schemaName, model.Group{}.TableName())
					integrationutils.CheckRegion(ctx, t, testConfig.DB, validTenantID, tt.region)
				}
			},
		)
	}
}

func TestHandleCreateTenantConcurrent(t *testing.T) {
	ctx := t.Context()
	handler := slogctx.NewHandler(
		slog.NewTextHandler(
			os.Stdout, &slog.HandlerOptions{
				AddSource: false,
			},
		), nil,
	)

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Initialize TenantOperator
	testConfig := newTestOperator(t)

	validTenantID := uuid.NewString()
	validData, err := createValidTenantData(validTenantID, "", "")
	require.NoError(t, err)

	taskID := uuid.New()

	var (
		wg          sync.WaitGroup
		numRoutines = 4
	)

	errs := make(chan error, numRoutines)
	resps := make(chan orbital.HandlerResponse, numRoutines)

	for i := range numRoutines {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			rCtx := slogctx.Prepend(ctx, "Routine:", i)
			req := buildRequest(taskID, tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), validData)

			resp, opErr := testConfig.TenantOperator.HandleCreateTenant(rCtx, req)
			if opErr != nil {
				errs <- fmt.Errorf("onboarding failed: %w", opErr)
			} else {
				errs <- nil
			}

			resps <- resp
		}(i)
	}

	wg.Wait()
	close(errs)

	var errorCount int

	for err = range errs {
		if err != nil {
			t.Logf("error: %v", err)

			errorCount++
		}
	}

	assert.Equal(
		t, 0, errorCount, "Expected no errors. UniqueConstraint should be handled"+
			" in the CreateTenantSchema function",
	)

	integrationutils.GroupsExists(ctx, t, validTenantID, testConfig.DB)
}

func TestHandleApplyAuth_InvalidData(t *testing.T) {
	taskType := authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String()

	tests := []struct {
		name   string
		data   []byte
		expErr error
	}{
		{
			name:   "invalid proto data",
			data:   []byte("invalid-proto"),
			expErr: operator.ErrInvalidData,
		},
		{
			name: "missing tenant ID",
			data: func() []byte {
				data, err := proto.Marshal(&authgrpc.Auth{})
				assert.NoError(t, err)

				return data
			}(),
			expErr: operator.ErrInvalidTenantID,
		},
		{
			name: "invalid auth properties",
			data: func() []byte {
				data, err := proto.Marshal(
					&authgrpc.Auth{
						TenantId: uuid.NewString(),
					},
				)
				assert.NoError(t, err)

				return data
			}(),
			expErr: operator.ErrInvalidAuthProps,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				unusedDB := &multitenancy.DB{}
				_, clientCon := testutils.NewGRPCSuite(t)
				unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)
				unusedSMClient := sessionmanager.NewFakeSessionManagerClient()

				clientFactory := mockClient.NewMockFactory(
					registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
					sessionmanager.NewMockService(unusedSMClient),
				)

				responder := respondertest.NewResponder()
				target := orbital.OperatorTarget{
					Client: responder,
				}
				op, err := operator.NewTenantOperator(unusedDB, target, clientFactory, nil, nil)
				require.NoError(t, err)

				go func() {
					err = op.RunOperator(t.Context())
					assert.NoError(t, err)
				}()

				taskReq := orbital.TaskRequest{
					TaskID: uuid.New(),
					Type:   taskType,
					Data:   tt.data,
				}

				responder.NewRequest(taskReq)
				taskResp := responder.NewResponse()

				assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
				assert.Equal(t, string(orbital.TaskStatusFailed), taskResp.Status)
				assert.Contains(t, taskResp.ErrorMessage, tt.expErr.Error())
			},
		)
	}
}

func TestHandleApplyAuth_IssuerUpdate(t *testing.T) {
	taskType := authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String()

	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	t.Run(
		"should return failed task and not update issuer if tenant is not found", func(t *testing.T) {
			_, clientCon := testutils.NewGRPCSuite(t)
			unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)
			unusedSMClient := sessionmanager.NewFakeSessionManagerClient()
			responder := respondertest.NewResponder()
			operatorTarget := orbital.OperatorTarget{
				Client: responder,
			}

			clientFactory := mockClient.NewMockFactory(
				registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
				sessionmanager.NewMockService(unusedSMClient),
			)

			op, err := operator.NewTenantOperator(db, operatorTarget, clientFactory, nil, nil)
			require.NoError(t, err)

			go func() {
				err = op.RunOperator(t.Context())
				assert.NoError(t, err)
			}()

			auth := authgrpc.Auth{
				TenantId: uuid.NewString(),
				Properties: map[string]string{
					"issuer": "http://issuer-url",
				},
			}
			data, err := proto.Marshal(&auth)
			assert.NoError(t, err)

			taskReq := orbital.TaskRequest{
				TaskID: uuid.New(),
				Type:   taskType,
				Data:   data,
			}

			responder.NewRequest(taskReq)
			taskResp := responder.NewResponse()

			assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
			assert.Equal(t, string(orbital.TaskStatusFailed), taskResp.Status)
			assert.Contains(t, taskResp.ErrorMessage, operator.ErrFailedApplyOIDC.Error())
		},
	)
}

func TestHandleApplyAuth_SessionManagerResponse(t *testing.T) {
	taskType := authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String()

	tests := []struct {
		name               string
		expTaskResponse    orbital.TaskResponse
		sessionManagerResp *oidcmappinggrpc.ApplyOIDCMappingResponse
		sessionManagerErr  error
	}{
		{
			name: "should return task in progress when session manager returns error",
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 15,
			},
			sessionManagerErr: assert.AnError,
		},
		{
			name: "should return failed task when session manager returns unsuccessful response",
			expTaskResponse: orbital.TaskResponse{
				Status:       string(orbital.TaskStatusFailed),
				ErrorMessage: operator.ErrFailedApplyOIDC.Error(),
			},
			sessionManagerResp: &oidcmappinggrpc.ApplyOIDCMappingResponse{
				Success: false,
			},
		},
		{
			name: "should return done task and update issuer when session manager applies successfully",
			expTaskResponse: orbital.TaskResponse{
				Status: string(orbital.TaskStatusDone),
			},
			sessionManagerResp: &oidcmappinggrpc.ApplyOIDCMappingResponse{
				Success: true,
			},
		},
	}

	db, tenants, _ := testutils.NewTestDB(
		t,
		testutils.TestDBConfig{},
		testutils.WithGenerateTenants(len(tests)),
	)
	assert.Len(t, tenants, len(tests))

	r := sql.NewRepository(db)

	for i, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				_, clientCon := testutils.NewGRPCSuite(t)
				unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)
				sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
				responder := respondertest.NewResponder()
				operatorTarget := orbital.OperatorTarget{
					Client: responder,
				}

				clientFactory := mockClient.NewMockFactory(
					registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
					sessionmanager.NewMockService(sessionManagerClient),
				)
				op, err := operator.NewTenantOperator(db, operatorTarget, clientFactory, nil, nil)
				require.NoError(t, err)

				go func() {
					err = op.RunOperator(t.Context())
					assert.NoError(t, err)
				}()

				issuerURL := "http://issuer-url"
				auth := authgrpc.Auth{
					TenantId: tenants[i],
					Properties: map[string]string{
						"issuer": issuerURL,
					},
				}
				data, err := proto.Marshal(&auth)
				assert.NoError(t, err)

				taskReq := orbital.TaskRequest{
					TaskID: uuid.New(),
					Type:   taskType,
					Data:   data,
				}

				noOfCalls := 0
				sessionManagerClient.MockApplyOIDCMapping = func(
					_ context.Context,
					req *oidcmappinggrpc.ApplyOIDCMappingRequest,
				) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error) {
					assert.Equal(t, auth.GetTenantId(), req.GetTenantId())
					assert.Equal(t, auth.GetProperties()["issuer"], req.GetIssuer())

					noOfCalls++

					return tt.sessionManagerResp, tt.sessionManagerErr
				}

				responder.NewRequest(taskReq)
				taskResp := responder.NewResponse()

				assert.Equal(t, 1, noOfCalls)
				assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
				assert.Equal(t, tt.expTaskResponse.Status, taskResp.Status)
				assert.Equal(t, tt.expTaskResponse.ReconcileAfterSec, taskResp.ReconcileAfterSec)
				assert.Contains(t, taskResp.ErrorMessage, tt.expTaskResponse.ErrorMessage)

				tenant := &model.Tenant{
					ID: auth.GetTenantId(),
				}
				success, err := r.First(t.Context(), tenant, *repo.NewQuery())
				assert.NoError(t, err)
				assert.True(t, success)

				if tt.sessionManagerResp != nil && tt.sessionManagerResp.GetSuccess() {
					assert.Equal(t, issuerURL, tenant.IssuerURL)
					return
				}

				assert.Empty(t, tenant.IssuerURL)
			},
		)
	}
}

func TestHandleBlockTenant(t *testing.T) {
	unusedDB := &multitenancy.DB{}
	_, clientCon := testutils.NewGRPCSuite(t)
	unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)

	taskType := tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String()

	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: testutils.TestDB,
	}

	tenantManager, groupManager := createManagers(t, unusedDB, cfg)

	t.Run("should return failed task when tenant data is invalid", func(t *testing.T) {
		sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
		responder := respondertest.NewResponder()
		operatorTarget := orbital.OperatorTarget{
			Client: responder,
		}
		clientFactory := mockClient.NewMockFactory(
			registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
			sessionmanager.NewMockService(sessionManagerClient),
		)

		op, err := operator.NewTenantOperator(unusedDB, operatorTarget, clientFactory, tenantManager, groupManager)
		require.NoError(t, err)

		go func() {
			err = op.RunOperator(t.Context())
			assert.NoError(t, err)
		}()

		invalidData := []byte("invalid-proto")
		taskReq := orbital.TaskRequest{
			TaskID: uuid.New(),
			Type:   taskType,
			Data:   invalidData,
		}

		noOfCalls := 0
		sessionManagerClient.MockBlockOIDCMapping = func(
			_ context.Context,
			_ *oidcmappinggrpc.BlockOIDCMappingRequest,
		) (*oidcmappinggrpc.BlockOIDCMappingResponse, error) {
			noOfCalls++
			return &oidcmappinggrpc.BlockOIDCMappingResponse{}, nil
		}

		responder.NewRequest(taskReq)
		taskResp := responder.NewResponse()

		assert.Equal(t, 0, noOfCalls)
		assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
		assert.Equal(t, string(orbital.TaskStatusFailed), taskResp.Status)
		assert.Contains(t, taskResp.ErrorMessage, operator.ErrInvalidData.Error())
	})

	tests := []struct {
		name               string
		sessionManagerResp *oidcmappinggrpc.BlockOIDCMappingResponse
		sessionManagerErr  error
		expTaskResponse    orbital.TaskResponse
	}{
		{
			name:              "should return task in progress when session manager returns error",
			sessionManagerErr: assert.AnError,
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 15,
			},
		},
		{
			name: "should return failed task when session manager returns unsuccessful response",
			sessionManagerResp: &oidcmappinggrpc.BlockOIDCMappingResponse{
				Success: false,
			},
			expTaskResponse: orbital.TaskResponse{
				Status:       string(orbital.TaskStatusFailed),
				ErrorMessage: operator.ErrFailedResponse.Error(),
			},
		},
		{
			name: "should return done task when session manager blocks successfully",
			sessionManagerResp: &oidcmappinggrpc.BlockOIDCMappingResponse{
				Success: true,
			},
			expTaskResponse: orbital.TaskResponse{
				Status: string(orbital.TaskStatusDone),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
			responder := respondertest.NewResponder()
			target := orbital.OperatorTarget{
				Client: responder,
			}
			clientFactory := mockClient.NewMockFactory(
				registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
				sessionmanager.NewMockService(sessionManagerClient),
			)

			op, err := operator.NewTenantOperator(unusedDB, target, clientFactory, tenantManager, groupManager)
			require.NoError(t, err)

			go func() {
				err = op.RunOperator(t.Context())
				assert.NoError(t, err)
			}()

			tenant := tenantgrpc.Tenant{
				Id: uuid.NewString(),
			}
			data, err := proto.Marshal(&tenant)
			assert.NoError(t, err)

			taskReq := orbital.TaskRequest{
				TaskID: uuid.New(),
				Type:   taskType,
				Data:   data,
			}

			noOfCalls := 0
			sessionManagerClient.MockBlockOIDCMapping = func(
				_ context.Context,
				req *oidcmappinggrpc.BlockOIDCMappingRequest,
			) (*oidcmappinggrpc.BlockOIDCMappingResponse, error) {
				assert.Equal(t, tenant.GetId(), req.GetTenantId())

				noOfCalls++

				return tt.sessionManagerResp, tt.sessionManagerErr
			}

			responder.NewRequest(taskReq)
			taskResp := responder.NewResponse()

			assert.Equal(t, 1, noOfCalls)
			assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
			assert.Equal(t, tt.expTaskResponse.Status, taskResp.Status)
			assert.Equal(t, tt.expTaskResponse.ReconcileAfterSec, taskResp.ReconcileAfterSec)
			assert.Contains(t, taskResp.ErrorMessage, tt.expTaskResponse.ErrorMessage)
		})
	}
}

func TestHandleUnblockTenant(t *testing.T) {
	unusedDB := &multitenancy.DB{}
	_, clientCon := testutils.NewGRPCSuite(t)
	unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)

	taskType := tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String()

	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: testutils.TestDB,
	}

	tenantManager, groupManager := createManagers(t, unusedDB, cfg)

	t.Run("should return failed task when tenant data is invalid", func(t *testing.T) {
		sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
		responder := respondertest.NewResponder()
		operatorTarget := orbital.OperatorTarget{
			Client: responder,
		}
		clientFactory := mockClient.NewMockFactory(
			registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
			sessionmanager.NewMockService(sessionManagerClient),
		)

		op, err := operator.NewTenantOperator(unusedDB, operatorTarget, clientFactory, tenantManager, groupManager)
		require.NoError(t, err)

		go func() {
			err = op.RunOperator(t.Context())
			assert.NoError(t, err)
		}()

		invalidData := []byte("invalid-proto")
		taskReq := orbital.TaskRequest{
			TaskID: uuid.New(),
			Type:   taskType,
			Data:   invalidData,
		}

		noOfCalls := 0
		sessionManagerClient.MockUnblockOIDCMapping = func(
			_ context.Context,
			_ *oidcmappinggrpc.UnblockOIDCMappingRequest,
		) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error) {
			noOfCalls++
			return &oidcmappinggrpc.UnblockOIDCMappingResponse{}, nil
		}

		responder.NewRequest(taskReq)
		taskResp := responder.NewResponse()

		assert.Equal(t, 0, noOfCalls)
		assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
		assert.Equal(t, string(orbital.TaskStatusFailed), taskResp.Status)
		assert.Contains(t, taskResp.ErrorMessage, operator.ErrInvalidData.Error())
	})

	tests := []struct {
		name               string
		sessionManagerResp *oidcmappinggrpc.UnblockOIDCMappingResponse
		sessionManagerErr  error
		expTaskResponse    orbital.TaskResponse
	}{
		{
			name:              "should return task in progress when session manager returns error",
			sessionManagerErr: assert.AnError,
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 15,
			},
		},
		{
			name: "should return failed task when session manager returns unsuccessful response",
			sessionManagerResp: &oidcmappinggrpc.UnblockOIDCMappingResponse{
				Success: false,
			},
			expTaskResponse: orbital.TaskResponse{
				Status:       string(orbital.TaskStatusFailed),
				ErrorMessage: operator.ErrFailedResponse.Error(),
			},
		},
		{
			name: "should return done task when session manager unblocks successfully",
			sessionManagerResp: &oidcmappinggrpc.UnblockOIDCMappingResponse{
				Success: true,
			},
			expTaskResponse: orbital.TaskResponse{
				Status: string(orbital.TaskStatusDone),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
			responder := respondertest.NewResponder()
			target := orbital.OperatorTarget{
				Client: responder,
			}
			clientFactory := mockClient.NewMockFactory(
				registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
				sessionmanager.NewMockService(sessionManagerClient),
			)

			op, err := operator.NewTenantOperator(unusedDB, target, clientFactory, tenantManager, groupManager)
			require.NoError(t, err)

			go func() {
				err = op.RunOperator(t.Context())
				assert.NoError(t, err)
			}()

			tenant := tenantgrpc.Tenant{
				Id: uuid.NewString(),
			}
			data, err := proto.Marshal(&tenant)
			assert.NoError(t, err)

			taskReq := orbital.TaskRequest{
				TaskID: uuid.New(),
				Type:   taskType,
				Data:   data,
			}

			noOfCalls := 0
			sessionManagerClient.MockUnblockOIDCMapping = func(
				_ context.Context,
				req *oidcmappinggrpc.UnblockOIDCMappingRequest,
			) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error) {
				assert.Equal(t, tenant.GetId(), req.GetTenantId())

				noOfCalls++

				return tt.sessionManagerResp, tt.sessionManagerErr
			}

			responder.NewRequest(taskReq)
			taskResp := responder.NewResponse()

			assert.Equal(t, 1, noOfCalls)
			assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
			assert.Equal(t, tt.expTaskResponse.Status, taskResp.Status)
			assert.Equal(t, tt.expTaskResponse.ReconcileAfterSec, taskResp.ReconcileAfterSec)
			assert.Contains(t, taskResp.ErrorMessage, tt.expTaskResponse.ErrorMessage)
		})
	}
}

func TestHandleTerminateTenant_RemoveAuth(t *testing.T) {
	unusedDB := &multitenancy.DB{}
	_, clientCon := testutils.NewGRPCSuite(t)
	unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)
	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: testutils.TestDB,
	}
	_, groupManager := createManagers(t, unusedDB, cfg)
	mockTenantManager := &MockTenantManager{}

	taskType := tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String()

	t.Run("should return failed task when tenant data is invalid", func(t *testing.T) {
		sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
		responder := respondertest.NewResponder()
		target := orbital.OperatorTarget{
			Client: responder,
		}
		clientFactory := mockClient.NewMockFactory(
			registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
			sessionmanager.NewMockService(sessionManagerClient),
		)

		op, err := operator.NewTenantOperator(unusedDB, target, clientFactory, mockTenantManager, groupManager)
		require.NoError(t, err)

		go func() {
			err = op.RunOperator(t.Context())
			assert.NoError(t, err)
		}()

		invalidData := []byte("invalid-proto")
		taskReq := orbital.TaskRequest{
			TaskID: uuid.New(),
			Type:   taskType,
			Data:   invalidData,
		}

		noOfCalls := 0
		sessionManagerClient.MockRemoveOIDCMapping = func(
			_ context.Context,
			_ *oidcmappinggrpc.RemoveOIDCMappingRequest,
		) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
			noOfCalls++
			return &oidcmappinggrpc.RemoveOIDCMappingResponse{}, nil
		}

		responder.NewRequest(taskReq)
		taskResp := responder.NewResponse()

		assert.Equal(t, 0, noOfCalls)
		assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
		assert.Equal(t, string(orbital.TaskStatusFailed), taskResp.Status)
		assert.Contains(t, taskResp.ErrorMessage, operator.ErrInvalidData.Error())
	})

	tests := []struct {
		name               string
		sessionManagerResp *oidcmappinggrpc.RemoveOIDCMappingResponse
		sessionManagerErr  error
		expTaskResponse    orbital.TaskResponse
	}{
		{
			name: "should return task in progress when session manager returns error",
			sessionManagerResp: &oidcmappinggrpc.RemoveOIDCMappingResponse{
				Success: true,
			},
			sessionManagerErr: assert.AnError,
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 15,
			},
		},
		{
			name: "should return failed task when session manager returns unsuccessful response",
			sessionManagerResp: &oidcmappinggrpc.RemoveOIDCMappingResponse{
				Success: false,
			},
			expTaskResponse: orbital.TaskResponse{
				Status:       string(orbital.TaskStatusFailed),
				ErrorMessage: operator.ErrFailedResponse.Error(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
			responder := respondertest.NewResponder()
			target := orbital.OperatorTarget{
				Client: responder,
			}
			clientFactory := mockClient.NewMockFactory(
				registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
				sessionmanager.NewMockService(sessionManagerClient),
			)

			op, err := operator.NewTenantOperator(unusedDB, target, clientFactory, mockTenantManager, groupManager)
			require.NoError(t, err)

			go func() {
				err = op.RunOperator(t.Context())
				assert.NoError(t, err)
			}()

			tenant := tenantgrpc.Tenant{
				Id: uuid.NewString(),
			}
			data, err := proto.Marshal(&tenant)
			assert.NoError(t, err)

			taskReq := orbital.TaskRequest{
				TaskID: uuid.New(),
				Type:   taskType,
				Data:   data,
			}

			noOfCalls := 0
			sessionManagerClient.MockRemoveOIDCMapping = func(
				_ context.Context,
				req *oidcmappinggrpc.RemoveOIDCMappingRequest,
			) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
				assert.Equal(t, tenant.GetId(), req.GetTenantId())

				noOfCalls++

				return tt.sessionManagerResp, tt.sessionManagerErr
			}
			mockTenantManager.mockOffboardTenant = func(_ context.Context) (manager.OffboardingResult, error) {
				return manager.OffboardingResult{Status: manager.OffboardingSuccess}, nil
			}

			responder.NewRequest(taskReq)
			taskResp := responder.NewResponse()

			assert.Equal(t, 1, noOfCalls)
			assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
			assert.Equal(t, tt.expTaskResponse.Status, taskResp.Status)
			assert.Equal(t, tt.expTaskResponse.ReconcileAfterSec, taskResp.ReconcileAfterSec)
			assert.Contains(t, taskResp.ErrorMessage, tt.expTaskResponse.ErrorMessage)
		},
		)
	}
}

func TestHandleTerminateTenant(t *testing.T) {
	unusedDB := &multitenancy.DB{}
	_, clientCon := testutils.NewGRPCSuite(t)
	unusedRegistryClient := tenantgrpc.NewServiceClient(clientCon)
	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: testutils.TestDB,
	}

	sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()
	sessionManagerClient.MockRemoveOIDCMapping = func(
		_ context.Context,
		_ *oidcmappinggrpc.RemoveOIDCMappingRequest,
	) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
		return &oidcmappinggrpc.RemoveOIDCMappingResponse{
			Success: true,
		}, nil
	}
	clientFactory := mockClient.NewMockFactory(
		registry.NewMockService(nil, unusedRegistryClient, mappingv1.NewServiceClient(clientCon)),
		sessionmanager.NewMockService(sessionManagerClient),
	)

	_, groupManager := createManagers(t, unusedDB, cfg)
	mockTenantManager := MockTenantManager{}

	taskType := tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String()

	tests := []struct {
		name            string
		expTaskResponse orbital.TaskResponse
		expDeleteCalls  int
		status          manager.OffboardingStatus
		offboardingErr  error
		deleteErr       error
	}{
		{
			name: "should return failed task and not delete tenant when offboarding fails",
			expTaskResponse: orbital.TaskResponse{
				Status:       string(orbital.TaskStatusFailed),
				ErrorMessage: operator.ErrTenantOffboarding.Error(),
			},
			expDeleteCalls: 0,
			status:         manager.OffboardingFailed,
		},
		{
			name: "should return task in progress and not delete tenant when offboarding is processing",
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 3,
			},
			expDeleteCalls: 0,
			status:         manager.OffboardingProcessing,
		},
		{
			name: "should return task in progress and not delete tenant when offboarding returns error",
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 15,
			},
			expDeleteCalls: 0,
			offboardingErr: assert.AnError,
		},
		{
			name: "should return task in progress when delete returns error",
			expTaskResponse: orbital.TaskResponse{
				Status:            string(orbital.TaskStatusProcessing),
				ReconcileAfterSec: 15,
			},
			expDeleteCalls: 1,
			status:         manager.OffboardingSuccess,
			deleteErr:      assert.AnError,
		},
		{
			name: "should return done task when delete is successful",
			expTaskResponse: orbital.TaskResponse{
				Status: string(orbital.TaskStatusDone),
			},
			expDeleteCalls: 1,
			status:         manager.OffboardingSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expTenantID := uuid.NewString()

			responder := respondertest.NewResponder()
			target := orbital.OperatorTarget{
				Client: responder,
			}
			op, err := operator.NewTenantOperator(
				unusedDB,
				target,
				clientFactory,
				&mockTenantManager,
				groupManager,
			)
			require.NoError(t, err)

			go func() {
				err = op.RunOperator(t.Context())
				assert.NoError(t, err)
			}()

			tenant := tenantgrpc.Tenant{
				Id: expTenantID,
			}
			data, err := proto.Marshal(&tenant)
			assert.NoError(t, err)

			taskReq := orbital.TaskRequest{
				TaskID: uuid.New(),
				Type:   taskType,
				Data:   data,
			}

			noOfCalls := 0
			mockTenantManager.mockOffboardTenant = func(ctx context.Context) (manager.OffboardingResult, error) {
				id, err := cmkcontext.ExtractTenantID(ctx)
				assert.Equal(t, expTenantID, id)
				assert.NoError(t, err)

				noOfCalls++

				return manager.OffboardingResult{
					Status: tt.status,
				}, tt.offboardingErr
			}

			noOfDeleteCalls := 0
			mockTenantManager.mockDeleteTenant = func(ctx context.Context) error {
				id, err := cmkcontext.ExtractTenantID(ctx)
				assert.Equal(t, expTenantID, id)
				assert.NoError(t, err)

				noOfDeleteCalls++

				return tt.deleteErr
			}

			responder.NewRequest(taskReq)
			taskResp := responder.NewResponse()

			assert.Equal(t, 1, noOfCalls)
			assert.Equal(t, tt.expDeleteCalls, noOfDeleteCalls)
			assert.Equal(t, taskReq.TaskID, taskResp.TaskID)
			assert.Equal(t, tt.expTaskResponse.Status, taskResp.Status)
			assert.Equal(t, tt.expTaskResponse.ReconcileAfterSec, taskResp.ReconcileAfterSec)
			assert.Contains(t, taskResp.ErrorMessage, tt.expTaskResponse.ErrorMessage)
		})
	}
}

func TestExtractOIDCConfig(t *testing.T) {
	tests := []struct {
		name           string
		properties     map[string]string
		expectedConfig operator.OIDCConfig
		expectError    bool
	}{
		{
			name: "valid properties",
			properties: map[string]string{
				"issuer":    "https://test.issuer.com",
				"jwks_uri":  "https://test.jwks1.com",
				"audiences": "audience1",
			},
			expectedConfig: operator.OIDCConfig{
				Issuer:               "https://test.issuer.com",
				JwksURI:              "https://test.jwks1.com",
				Audiences:            []string{"audience1"},
				AdditionalProperties: map[string]string{},
			},
		},
		{
			name: "valid properties and additional properties",
			properties: map[string]string{
				"issuer":    "https://test.issuer.com",
				"jwks_uri":  "https://test.jwks1.com",
				"audiences": "audience1",
				"client-id": "some-client-id",
			},
			expectedConfig: operator.OIDCConfig{
				Issuer:    "https://test.issuer.com",
				JwksURI:   "https://test.jwks1.com",
				Audiences: []string{"audience1"},
				AdditionalProperties: map[string]string{
					"client-id": "some-client-id",
				},
			},
		},
		{
			name: "missing optional properties",
			properties: map[string]string{
				"issuer": "https://test.issuer.com",
			},
			expectedConfig: operator.OIDCConfig{
				Issuer:               "https://test.issuer.com",
				JwksURI:              "",
				Audiences:            []string{},
				AdditionalProperties: map[string]string{},
			},
		},
		{
			name:        "missing issuer property",
			properties:  map[string]string{},
			expectError: true,
		},
		{
			name: "empty issuer property",
			properties: map[string]string{
				"issuer": "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				config, err := operator.ExtractOIDCConfig(tt.properties)

				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedConfig, config)
				}
			},
		)
	}
}

func TestParseCommaSeparatedValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single value",
			input:    "value1",
			expected: []string{"value1"},
		},
		{
			name:     "single value with spaces",
			input:    "  value1  ",
			expected: []string{"value1"},
		},
		{
			name:     "multiple values",
			input:    "value1,value2,value3",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "multiple values with spaces",
			input:    " value1 , value2 , value3 ",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "values with empty entries",
			input:    "value1,,value2,",
			expected: []string{"value1", "value2"},
		},
		{
			name:     "values with empty entries and spaces",
			input:    " value1 ,, value2 , ",
			expected: []string{"value1", "value2"},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: []string{},
		},
		{
			name:     "only spaces and commas",
			input:    " , , , ",
			expected: []string{},
		},
		{
			name:     "URL values",
			input:    "https://test1.com,https://test2.com",
			expected: []string{"https://test1.com", "https://test2.com"},
		},
		{
			name:     "mixed content with empty values",
			input:    "audience1,,audience2,audience3,",
			expected: []string{"audience1", "audience2", "audience3"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := operator.ParseCommaSeparatedValues(tt.input)
				assert.Equal(t, tt.expected, result)
			},
		)
	}
}

type TestConfig struct {
	TenantOperator           *operator.TenantOperator
	DB                       *multitenancy.DB
	FakeTenantService        *tenants.FakeTenantService
	FakeSessionManagerClient *sessionmanager.FakeSessionManagerClient
	TenantIDs                []string
}

// newTestOperator creates and returns a new instance of TenantOperator along with its dependencies
// for use in tests. It sets up a test database, AMQP client, fake tenant service, and fake session manager client.
// Returns:
//   - *operator.TenantOperator: the initialized TenantOperator
//   - *multitenancy.DB: the test multitenancy database
//   - *tenants.FakeTenantService: the fake tenant service for testing
//   - *sessionmanager.FakeSessionManagerClient: the fake session manager client for testing
func newTestOperator(t *testing.T, opts ...testutils.TestDBConfigOpt) TestConfig {
	t.Helper()
	multitenancyDB, list, cfgDB := testutils.NewTestDB(
		t,
		testutils.TestDBConfig{CreateDatabase: true},
		opts...,
	)

	amqpClient, _ := testutils.NewAMQPClient(t, testutils.AMQPCfg{})

	fakeTenantService := tenants.NewFakeTenantService()
	_, grpcClient := testutils.NewGRPCSuite(
		t,
		func(s *grpc.Server) {
			tenantgrpc.RegisterServiceServer(s, fakeTenantService)
		},
	)

	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: cfgDB,
	}

	operatorTarget := orbital.OperatorTarget{
		Client: amqpClient,
	}
	tenantClient := tenantgrpc.NewServiceClient(grpcClient)
	sessionManagerClient := sessionmanager.NewFakeSessionManagerClient()

	clientFactory := mockClient.NewMockFactory(
		registry.NewMockService(nil, tenantClient, mappingv1.NewServiceClient(grpcClient)),
		sessionmanager.NewMockService(sessionManagerClient),
	)

	tenantManager, groupManager := createManagers(t, multitenancyDB, cfg)
	tenantOperator, err := operator.NewTenantOperator(
		multitenancyDB,
		operatorTarget,
		clientFactory,
		tenantManager,
		groupManager,
	)
	require.NoError(t, err, "Failed to create TenantOperator")
	require.NotNil(t, tenantOperator, "TenantOperator should not be nil")

	return TestConfig{
		tenantOperator, multitenancyDB, fakeTenantService,
		sessionManagerClient, list,
	}
}

// buildRequest creates a properly structured handler request with TaskID
func buildRequest(taskID uuid.UUID, actionType string, data []byte) orbital.HandlerRequest {
	return orbital.HandlerRequest{
		TaskID: taskID,
		Type:   actionType,
		Data:   data,
	}
}

// createValidTenantData is a helper to create valid tenant protobuf data
func createValidTenantData(tenantID, region, name string) ([]byte, error) {
	tenant := &tenantgrpc.Tenant{
		Id:     tenantID,
		Region: region,
		Name:   name,
	}

	return proto.Marshal(tenant)
}
