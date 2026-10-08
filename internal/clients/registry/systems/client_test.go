package systems_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	typesv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/testutils"
)

const (
	existingTenantID = "existing-tenant-id"
)

func TestRegistryService_SystemsClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: false}))
	systemService := systems.NewFakeService(logger)
	_, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
		},
	)

	systemsClient, err := systems.NewSystemsClient(grpcClient)
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("GetSystemsWithFilter", func(t *testing.T) {
		t.Run("should return an error if no entries exist", func(t *testing.T) {
			resp, err := systemsClient.GetSystemsWithFilter(ctx, systems.SystemFilter{})

			assert.ErrorContains(t, err, systems.ErrSystemsClientFailedGettingSystems.Error())
			assert.Nil(t, resp)
		})

		t.Run("when entries exist", func(t *testing.T) {
			// At present time there is no implementation for system registry
			// in order to not expose unnecessary endpoints on the client create an instace
			// of the base SystemClient
			baseSystemClient := systemgrpc.NewServiceClient(grpcClient)

			sysReq1 := validRegisterSystemReq(false)
			sysReq1.TenantId = existingTenantID
			_, err := baseSystemClient.RegisterSystem(ctx, sysReq1)
			assert.NoError(t, err)

			sysReq2 := validRegisterSystemReq(false)
			_, err = baseSystemClient.RegisterSystem(ctx, sysReq2)
			assert.NoError(t, err)

			t.Run("must not return any records if no filter is applied", func(t *testing.T) {
				_, err := systemsClient.GetSystemsWithFilter(ctx, systems.SystemFilter{})

				assert.Error(t, err)
			})

			t.Run("should return System filtered by", func(t *testing.T) {
				tests := []struct {
					name               string
					request            systems.SystemFilter
					expectedExternalID string
					expectedRegion     string
				}{
					{
						name: "TenantID",
						request: systems.SystemFilter{
							TenantID: sysReq1.GetTenantId(),
						},
						expectedExternalID: sysReq1.GetExternalId(),
						expectedRegion:     sysReq1.GetRegion(),
					},
					{
						name: "ExternalID and Region",
						request: systems.SystemFilter{
							ExternalID: sysReq2.GetExternalId(),
							Region:     sysReq2.GetRegion(),
						},
						expectedExternalID: sysReq2.GetExternalId(),
						expectedRegion:     sysReq2.GetRegion(),
					},
					{
						name: "TenantID and ExternalID",
						request: systems.SystemFilter{
							TenantID:   sysReq1.GetTenantId(),
							ExternalID: sysReq1.GetExternalId(),
						},
						expectedExternalID: sysReq1.GetExternalId(),
						expectedRegion:     sysReq1.GetRegion(),
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						sys, err := systemsClient.GetSystemsWithFilter(ctx, tt.request)

						assert.NoError(t, err)
						assert.Len(t, sys, 1)
						assert.Equal(t, tt.expectedExternalID, sys[0].Identifier)
						assert.Equal(t, tt.expectedRegion, sys[0].Region)
					})
				}
			})

			t.Run("should return an error", func(t *testing.T) {
				tests := []struct {
					name    string
					request systems.SystemFilter
				}{
					{
						name: "non-existent TenantID is provided",
						request: systems.SystemFilter{
							TenantID: uuid.NewString(),
						},
					},
					{
						name: "non-existent ExternalID is provided",
						request: systems.SystemFilter{
							ExternalID: randExternalID(),
						},
					},
					{
						name: "non-existent TenantID and ExternalID are provided",
						request: systems.SystemFilter{
							TenantID:   uuid.NewString(),
							ExternalID: randExternalID(),
						},
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						resp, err := systemsClient.GetSystemsWithFilter(ctx, tt.request)

						assert.Error(t, err)
						assert.ErrorContains(t, err, systems.ErrSystemsClientFailedGettingSystems.Error())
						assert.Nil(t, resp)
					})
				}
			})
		})
	})
	t.Run("ExtendedUpdateSystemL1KeyClaim", func(t *testing.T) {
		// At present time there is no implementation for system registry
		// in order to not expose unnecessary endpoints on the client create an instace
		// of the base SystemClient
		baseSystemClient := systemgrpc.NewServiceClient(grpcClient)

		t.Run("Should update", func(t *testing.T) {
			sysReq1 := validRegisterSystemReq(false)
			_, err := baseSystemClient.RegisterSystem(ctx, sysReq1)
			assert.NoError(t, err)

			err = systemsClient.ExtendedUpdateSystemL1KeyClaim(ctx, systems.SystemFilter{
				Region:     sysReq1.GetRegion(),
				ExternalID: sysReq1.GetExternalId(),
				TenantID:   sysReq1.GetTenantId(),
			}, true)
			assert.NoError(t, err)
		})
	})
	t.Run("ExtendedUpdateSystemStatus", func(t *testing.T) {
		baseSystemClient := systemgrpc.NewServiceClient(grpcClient)

		t.Run("Should update status to LOCKED", func(t *testing.T) {
			sysReq := &systemgrpc.RegisterSystemRequest{
				ExternalId: randExternalID(),
				Region:     "eu",
				Type:       "system",
			}
			_, err := baseSystemClient.RegisterSystem(ctx, sysReq)
			assert.NoError(t, err)

			filter := systems.SystemFilter{
				Region:     sysReq.GetRegion(),
				ExternalID: sysReq.GetExternalId(),
			}

			// First call applies the status; second call must succeed too — the
			// registry's UpdateSystemStatus is naturally idempotent.
			assert.NoError(t,
				systemsClient.ExtendedUpdateSystemStatus(ctx, filter, sysReq.GetType(), typesv1.Status_STATUS_LOCKED))
			assert.NoError(t,
				systemsClient.ExtendedUpdateSystemStatus(ctx, filter, sysReq.GetType(), typesv1.Status_STATUS_LOCKED))

			resp, err := baseSystemClient.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
				Region:     sysReq.GetRegion(),
				ExternalId: sysReq.GetExternalId(),
			})
			assert.NoError(t, err)
			assert.Equal(t, typesv1.Status_STATUS_LOCKED, resp.GetSystems()[0].GetStatus())
		})
	})
}

// failingUpdateStatusServer returns the configured error from UpdateSystemStatus.
type failingUpdateStatusServer struct {
	systemgrpc.UnimplementedServiceServer

	err error
}

func (s *failingUpdateStatusServer) UpdateSystemStatus(
	_ context.Context,
	_ *systemgrpc.UpdateSystemStatusRequest,
) (*systemgrpc.UpdateSystemStatusResponse, error) {
	return nil, s.err
}

func TestExtendedUpdateSystemStatus_PropagatesErrors(t *testing.T) {
	tests := []struct {
		name        string
		serverErr   error
		expectedErr error
	}{
		{
			name:        "Internal is wrapped with ErrClientInternalError",
			serverErr:   status.Error(codes.Internal, "boom"),
			expectedErr: systems.ErrClientInternalError,
		},
		{
			name:        "NotFound is wrapped with ErrSystemsClientFailedUpdatingStatus",
			serverErr:   status.Error(codes.NotFound, "system not found"),
			expectedErr: systems.ErrSystemsClientFailedUpdatingStatus,
		},
		{
			name:        "InvalidArgument is wrapped with ErrSystemsClientFailedUpdatingStatus",
			serverErr:   status.Error(codes.InvalidArgument, "bad status"),
			expectedErr: systems.ErrSystemsClientFailedUpdatingStatus,
		},
		{
			name:        "FailedPrecondition is wrapped with ErrSystemsClientFailedUpdatingStatus",
			serverErr:   status.Error(codes.FailedPrecondition, "precondition"),
			expectedErr: systems.ErrSystemsClientFailedUpdatingStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, grpcClient := testutils.NewGRPCSuite(t,
				func(s *grpc.Server) {
					systemgrpc.RegisterServiceServer(s, &failingUpdateStatusServer{err: tt.serverErr})
				},
			)

			c, err := systems.NewSystemsClient(grpcClient)
			require.NoError(t, err)

			err = c.ExtendedUpdateSystemStatus(t.Context(),
				systems.SystemFilter{Region: "eu", ExternalID: "ext", TenantID: existingTenantID},
				"system",
				typesv1.Status_STATUS_LOCKED,
			)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
