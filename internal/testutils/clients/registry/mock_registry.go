package registry

import (
	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
)

// MockServiceStruct is a mock implementation that can be used for testing purposes.
type MockServiceStruct struct {
	SystemClient  systems.ServiceClient
	TenantClient  tenantgrpc.ServiceClient
	MappingClient mappinggrpc.ServiceClient
}

var _ registry.Service = (*MockServiceStruct)(nil)

func NewMockService(
	system systems.ServiceClient,
	tenant tenantgrpc.ServiceClient,
	mapping mappinggrpc.ServiceClient,
) registry.Service {
	return &MockServiceStruct{
		SystemClient:  system,
		TenantClient:  tenant,
		MappingClient: mapping,
	}
}

func (rs *MockServiceStruct) System() systems.ServiceClient {
	return rs.SystemClient
}

func (rs *MockServiceStruct) Tenant() tenantgrpc.ServiceClient {
	return rs.TenantClient
}

func (rs *MockServiceStruct) Mapping() mappinggrpc.ServiceClient {
	return rs.MappingClient
}

func (rs *MockServiceStruct) Close() error {
	return nil
}
