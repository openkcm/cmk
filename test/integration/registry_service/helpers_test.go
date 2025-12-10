package registry_service_test

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	regionpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.tools.sap/kms/cmk/internal/errs"
)

var ErrUnknownClient = errors.New("client is unknown")

func validRandExternalID() string {
	id1 := strings.ReplaceAll(uuid.New().String(), "-", "")
	id2 := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]

	return id1 + id2
}

func validRegisterSystemReq() *systemgrpc.RegisterSystemRequest {
	return &systemgrpc.RegisterSystemRequest{
		ExternalId: validRandExternalID(),
		L2KeyId:    "key123",
		Region:     string(regionpb.Region_REGION_EU),
	}
}

func deleteResource(ctx context.Context, client any, id string) error {
	switch c := client.(type) {
	case tenantgrpc.ServiceClient:
		_, err := c.TerminateTenant(ctx, &tenantgrpc.TerminateTenantRequest{
			Id: id,
		})
		if err != nil {
			return errs.Wrapf(err, "error deleting system with tenants client")
		}

		return nil

	case systemgrpc.ServiceClient:
		_, err := c.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
			ExternalId: id,
		})
		if err != nil {
			return errs.Wrapf(err, "error deleting system with service client")
		}

		return nil

	default:
		return ErrUnknownClient
	}
}

func unlinkTenant(ctx context.Context, subj any, id string) error {
	s, ok := subj.(systemgrpc.ServiceClient)
	if !ok {
		return ErrUnknownClient
	}

	_, err := s.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
		SystemIdentifiers: []*systemgrpc.SystemIdentifier{{ExternalId: id}},
	})
	if err != nil {
		return errs.Wrapf(err, "error unlinking tenant with service client")
	}

	return nil
}
