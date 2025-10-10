package operator_test

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/clients/registry/tenants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/tenant-manager/internal/operator"
	integrationutils "github.com/openkcm/cmk/test/integration_utils"
	tmdb "github.com/openkcm/cmk/utils/base62"
)

const (
	RegionUSWest1 = "us-west-1"
	AMQPURL       = "amqp://guest:guest@localhost:5672"
	AMQPTarget    = "cmk.global.tenants"
	AMQPSource    = "cmk.emea.tenants"
)

func TestNewTenantOperator(t *testing.T) {
	amqpClient := &amqp.AMQP{}
	dbConn := &multitenancy.DB{}
	fts := tenants.NewFakeTenantService()

	server, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			tenantgrpc.RegisterServiceServer(s, fts)
		})
	defer server.Stop()
	defer func(grpcClient *commongrpc.DynamicClientConn) {
		err := grpcClient.Close()
		if err != nil {
			t.Logf("Failed to close gRPC client: %v", err)
		}
	}(grpcClient)

	tenantClient := tenantgrpc.NewServiceClient(grpcClient)

	t.Run("nil db", func(t *testing.T) {
		op, err := operator.NewTenantOperator(nil, amqpClient, tenantClient)
		assert.Nil(t, op)
		assert.Error(t, err)
	})

	t.Run("nil amqp", func(t *testing.T) {
		op, err := operator.NewTenantOperator(dbConn, nil, tenantClient)
		assert.Nil(t, op)
		assert.Error(t, err)
	})

	t.Run("nil tenant service", func(t *testing.T) {
		op, err := operator.NewTenantOperator(dbConn, amqpClient, nil)
		assert.Nil(t, op)
		assert.Error(t, err)
	})

	t.Run("valid operator", func(t *testing.T) {
		op, err := operator.NewTenantOperator(dbConn, amqpClient, tenantClient)
		require.NoError(t, err)
		assert.NotNil(t, op)
	})
}

func TestRunOperator(t *testing.T) {
	tenantOperator, _, _ := newTestOperator(t)

	err := tenantOperator.RunOperator(t.Context())
	require.NoError(t, err)
}

func TestHandleCreateTenant(t *testing.T) {
	// Initialize TenantOperator
	ctx := t.Context()
	tenantOperator, multitenancyDB, fakeTenantService := newTestOperator(t)

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
				_, err = tenantOperator.HandleCreateTenant(ctx, req)
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
				_, err = tenantOperator.HandleCreateTenant(ctx, req)
				require.NoError(t, err)

				// Configure the fake service to fail on the second call
				fakeTenantService.SetTenantUserGroupsError = operator.ErrSendingGroupsFailed
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
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			req := buildRequest(uuid.New(), tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), tt.data)
			resp, err := tenantOperator.HandleCreateTenant(ctx, req)

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
				integrationutils.TenantExists(t, multitenancyDB, schemaName, model.Group{}.TableName())
				integrationutils.CheckRegion(ctx, t, multitenancyDB, validTenantID, tt.region)
			}
		})
	}
}

func TestHandleCreateTenantConcurrent(t *testing.T) {
	ctx := t.Context()
	handler := slogctx.NewHandler(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
		}), nil,
	)

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Initialize TenantOperator
	tenantOperator, multitenancyDB, _ := newTestOperator(t)

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

			resp, opErr := tenantOperator.HandleCreateTenant(rCtx, req)
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

	assert.Equal(t, 0, errorCount, "Expected no errors. UniqueConstraint should be handled"+
		" in the CreateTenantSchema function")

	integrationutils.GroupsExists(ctx, t, validTenantID, multitenancyDB)
}

// TestHandleBlockTenant is a test that verifies HandleBlockTenant returns ResultFailed.
func TestHandleBlockTenant(t *testing.T) {
	resp, err := operator.HandleBlockTenant(t.Context(), orbital.HandlerRequest{})
	require.NoError(t, err)
	assert.Equal(t, orbital.ResultFailed, resp.Result)
	assert.Equal(t, operator.WorkingStateToBeImplemented, string(resp.WorkingState))
}

// TestHandleUnblockTenant is a test that verifies HandleUnblockTenant returns ResultFailed.
func TestHandleUnblockTenant(t *testing.T) {
	resp, err := operator.HandleUnblockTenant(t.Context(), orbital.HandlerRequest{})
	require.NoError(t, err)
	assert.Equal(t, orbital.ResultFailed, resp.Result)
	assert.Equal(t, operator.WorkingStateToBeImplemented, string(resp.WorkingState))
}

// TestHandleTerminateTenant is a test that verifies HandleTerminateTenant returns ResultFailed.
func TestHandleTerminateTenant(t *testing.T) {
	resp, err := operator.HandleTerminateTenant(t.Context(), orbital.HandlerRequest{})
	require.NoError(t, err)
	assert.Equal(t, orbital.ResultFailed, resp.Result)
	assert.Equal(t, operator.WorkingStateToBeImplemented, string(resp.WorkingState))
}

// TestHandleApplyTenantAuth is a test that verifies HandleApplyTenantAuth returns ResultFailed.
func TestHandleApplyTenantAuth(t *testing.T) {
	resp, err := operator.HandleApplyTenantAuth(t.Context(), orbital.HandlerRequest{})
	require.NoError(t, err)
	assert.Equal(t, orbital.ResultFailed, resp.Result)
	assert.Equal(t, operator.WorkingStateToBeImplemented, string(resp.WorkingState))
}

// newTestOperator creates a new TenantOperator for testing purposes.
func newTestOperator(t *testing.T) (*operator.TenantOperator, *multitenancy.DB, *tenants.FakeTenantService) {
	t.Helper()
	multitenancyDB, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.Tenant{}, &model.Group{}},
	})

	amqpClient, err := amqp.NewClient(t.Context(), codec.Proto{}, amqp.ConnectionInfo{
		URL:    AMQPURL,
		Target: AMQPTarget,
		Source: AMQPSource,
	})
	require.NoError(t, err, "Failed to create AMQP client")

	fakeTenantService := tenants.NewFakeTenantService()
	server, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			tenantgrpc.RegisterServiceServer(s, fakeTenantService)
		},
	)

	t.Cleanup(func() {
		server.Stop()

		err = grpcClient.Close()
		if err != nil {
			t.Logf("Failed to close gRPC client: %v", err)
		}
	})

	tenantClient := tenantgrpc.NewServiceClient(grpcClient)

	tenantOperator, err := operator.NewTenantOperator(multitenancyDB, amqpClient, tenantClient)
	require.NoError(t, err, "Failed to create TenantOperator")
	require.NotNil(t, tenantOperator, "TenantOperator should not be nil")

	return tenantOperator, multitenancyDB, fakeTenantService
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
