package sessionmanager

import (
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	sessionmanager "github.tools.sap/kms/cmk/internal/clients/session-manager"
)

// MockServiceStruct is a mock implementation
// that can be used for testing purposes.
type MockServiceStruct struct {
	Client oidcmappinggrpc.ServiceClient
}

var _ sessionmanager.Service = (*MockServiceStruct)(nil)

func NewMockService(oidc oidcmappinggrpc.ServiceClient) sessionmanager.Service {
	return &MockServiceStruct{
		Client: oidc,
	}
}

func (c *MockServiceStruct) OIDCMapping() oidcmappinggrpc.ServiceClient {
	return c.Client
}

func (c *MockServiceStruct) Close() error {
	return nil
}
