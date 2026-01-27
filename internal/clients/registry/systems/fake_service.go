package systems

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	systemv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.com/openkcm/cmk/internal/errs"
)

type FakeService struct {
	systemv1.UnimplementedServiceServer

	log     *slog.Logger
	systems []*systemv1.System
}

type FakeServiceOption func(*FakeService)

func NewFakeService(log *slog.Logger, opts ...FakeServiceOption) *FakeService {
	fs := &FakeService{log: log}

	for _, o := range opts {
		o(fs)
	}

	fs.mustEmbedUnimplementedServiceServer()

	return fs
}

func WithSystems(systems ...*systemv1.System) FakeServiceOption {
	return func(fs *FakeService) {
		fs.systems = append(fs.systems, systems...)
	}
}

func (fs *FakeService) RegisterSystem(
	_ context.Context,
	req *systemv1.RegisterSystemRequest,
) (*systemv1.RegisterSystemResponse, error) {
	system := &systemv1.System{
		ExternalId:    req.GetExternalId(),
		TenantId:      req.GetTenantId(),
		L2KeyId:       req.GetL2KeyId(),
		HasL1KeyClaim: req.GetHasL1KeyClaim(),
		Region:        req.GetRegion(),
		Type:          req.GetType(),
	}

	fs.systems = append(fs.systems, system)

	return &systemv1.RegisterSystemResponse{}, nil
}

func (fs *FakeService) ListSystems(
	_ context.Context,
	req *systemv1.ListSystemsRequest,
) (*systemv1.ListSystemsResponse, error) {
	dummyPageToken := "EmptyPageToken"

	if req.GetPageToken() == dummyPageToken {
		return &systemv1.ListSystemsResponse{}, nil
	}

	err := fs.validateListRequest(req)
	if err != nil {
		return &systemv1.ListSystemsResponse{}, err
	}

	filteredSystems := fs.filterSystems(req)

	if len(filteredSystems) == 0 {
		return &systemv1.ListSystemsResponse{},
			errs.Wrapf(status.Error(codes.NotFound, "systems not found"), "error listing systems")
	}

	return &systemv1.ListSystemsResponse{
		Systems:       filteredSystems,
		NextPageToken: dummyPageToken,
	}, nil
}

func (fs *FakeService) DeleteSystem(
	_ context.Context,
	_ *systemv1.DeleteSystemRequest,
) (*systemv1.DeleteSystemResponse, error) {
	return &systemv1.DeleteSystemResponse{
		Success: true,
	}, nil
}

func (fs *FakeService) UpdateSystemL1KeyClaim(
	_ context.Context,
	in *systemv1.UpdateSystemL1KeyClaimRequest,
) (*systemv1.UpdateSystemL1KeyClaimResponse, error) {
	for _, system := range fs.systems {
		if system.GetExternalId() == in.GetExternalId() {
			system.HasL1KeyClaim = in.GetL1KeyClaim()
		}
	}

	return &systemv1.UpdateSystemL1KeyClaimResponse{Success: true}, nil
}

func (fs *FakeService) mustEmbedUnimplementedServiceServer() {}

func (fs *FakeService) validateListRequest(req *systemv1.ListSystemsRequest) error {
	if req.GetTenantId() == "" && (req.GetExternalId() == "" || req.GetRegion() == "") {
		return errs.Wrapf(status.Error(codes.InvalidArgument, "Too few arguments"), "error listing systems")
	}

	return nil
}

func (fs *FakeService) filterSystems(req *systemv1.ListSystemsRequest) []*systemv1.System {
	filteredSystems := make([]*systemv1.System, 0)

	for _, system := range fs.systems {
		if fs.systemMatchesRequest(system, req) {
			filteredSystems = append(filteredSystems, system)
		}
	}

	return filteredSystems
}

func (fs *FakeService) systemMatchesRequest(system *systemv1.System, req *systemv1.ListSystemsRequest) bool {
	if req.GetTenantId() != "" && system.GetTenantId() != req.GetTenantId() {
		return false
	}

	if req.GetExternalId() != "" && system.GetExternalId() != req.GetExternalId() {
		return false
	}

	if req.GetRegion() != "" && system.GetRegion() != req.GetRegion() {
		return false
	}

	return true
}
