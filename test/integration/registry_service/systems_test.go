package registry_service_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.tools.sap/kms/cmk/internal/clients/registry/systems"
	"github.tools.sap/kms/cmk/internal/testutils"
)

const (
	grpcServer       = "localhost"
	grpcPort         = "8010"
	existingTenantID = "existing-tenant-id"
)

func TestRegistryService_SystemsClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: false}))
	systemService := systems.NewFakeService(logger)
	_, conn := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
		},
	)

	defer conn.Close()

	systemsClient := systemgrpc.NewServiceClient(conn)
	ctx := t.Context()

	t.Run("ListSystems", func(t *testing.T) {
		t.Run("should return an error if no entries exist", func(t *testing.T) {
			// when
			resp, err := systemsClient.ListSystems(ctx, &systemgrpc.ListSystemsRequest{})

			// then
			assert.Error(t, err)
			assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			assert.Nil(t, resp)
		})

		t.Run("when entries exist", func(t *testing.T) {
			// given
			sysReq1 := validRegisterSystemReq()
			sysReq1.TenantId = existingTenantID
			_, err := systemsClient.RegisterSystem(ctx, sysReq1)
			assert.NoError(t, err)

			defer func() {
				err := unlinkTenant(ctx, systemsClient, sysReq1.GetExternalId())
				assert.NoError(t, err)
				err = deleteResource(ctx, systemsClient, sysReq1.GetExternalId())
				assert.NoError(t, err)
			}()

			sysReq2 := validRegisterSystemReq()
			_, err = systemsClient.RegisterSystem(ctx, sysReq2)
			assert.NoError(t, err)

			defer func() {
				err := deleteResource(ctx, systemsClient, sysReq2.GetExternalId())
				assert.NoError(t, err)
			}()

			t.Run("should return all records if no filter is applied", func(t *testing.T) {
				// when
				resp, err := systemsClient.ListSystems(ctx, &systemgrpc.ListSystemsRequest{})

				// then
				assert.NoError(t, err)
				assert.Len(t, resp.GetSystems(), 2)
			})

			t.Run("should return System filtered by", func(t *testing.T) {
				// given
				tests := []struct {
					name               string
					request            *systemgrpc.ListSystemsRequest
					expectedExternalID string
				}{
					{
						name: "TenantID",
						request: &systemgrpc.ListSystemsRequest{
							TenantId: sysReq1.GetTenantId(),
						},
						expectedExternalID: sysReq1.GetExternalId(),
					},
					{
						name: "ExternalID",
						request: &systemgrpc.ListSystemsRequest{
							ExternalId: sysReq2.GetExternalId(),
						},
						expectedExternalID: sysReq2.GetExternalId(),
					},
					{
						name: "TenantID and ExternalID",
						request: &systemgrpc.ListSystemsRequest{
							TenantId:   sysReq1.GetTenantId(),
							ExternalId: sysReq1.GetExternalId(),
						},
						expectedExternalID: sysReq1.GetExternalId(),
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						// when
						resp, err := systemsClient.ListSystems(ctx, tt.request)
						// then
						assert.NoError(t, err)
						assert.Len(t, resp.GetSystems(), 1)
						assert.Equal(t, tt.expectedExternalID, resp.GetSystems()[0].GetExternalId())
					})
				}
			})

			t.Run("should return an error if", func(t *testing.T) {
				// given
				tests := []struct {
					name    string
					request *systemgrpc.ListSystemsRequest
				}{
					{
						name: "non-existent TenantID is provided",
						request: &systemgrpc.ListSystemsRequest{
							TenantId: uuid.NewString(),
						},
					},
					{
						name: "non-existent ExternalID is provided",
						request: &systemgrpc.ListSystemsRequest{
							ExternalId: validRandExternalID(),
						},
					},
					{
						name: "non-existent TenantID and ExternalID are provided",
						request: &systemgrpc.ListSystemsRequest{
							TenantId:   uuid.NewString(),
							ExternalId: validRandExternalID(),
						},
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						// when
						resp, err := systemsClient.ListSystems(ctx, tt.request)
						// then
						assert.Error(t, err)
						assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
						assert.Nil(t, resp)
					})
				}
			})
		})
	})
}
