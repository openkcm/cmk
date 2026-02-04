package mapping

import (
	"context"

	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
)

type FakeService struct {
	mappingv1.UnimplementedServiceServer
}

func NewFakeService() *FakeService {
	fs := &FakeService{}
	fs.mustEmbedUnimplementedServiceServer()
	return fs
}

func (s *FakeService) UnmapSystemFromTenant(
	context.Context,
	*mappingv1.UnmapSystemFromTenantRequest,
) (*mappingv1.UnmapSystemFromTenantResponse, error) {
	// This method is intentionally left empty as it is a fake implementation for testing purposes.
	return nil, nil //nolint:nilnil
}

func (s *FakeService) MapSystemToTenant(
	context.Context,
	*mappingv1.MapSystemToTenantRequest,
) (*mappingv1.MapSystemToTenantResponse, error) {
	// This method is intentionally left empty as it is a fake implementation for testing purposes.
	return nil, nil //nolint:nilnil
}

func (s *FakeService) Get(
	context.Context,
	*mappingv1.GetRequest,
) (*mappingv1.GetResponse, error) {
	// This method is intentionally left empty as it is a fake implementation for testing purposes.
	return nil, nil //nolint:nilnil
}

func (s *FakeService) mustEmbedUnimplementedServiceServer() {}
