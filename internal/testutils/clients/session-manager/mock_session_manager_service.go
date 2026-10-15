package sessionmanager

import (
	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"

	sessionmanager "github.com/openkcm/cmk/internal/clients/session-manager"
)

// MockServiceStruct is a mock implementation
// that can be used for testing purposes.
type MockServiceStruct struct {
	Client trustmappingv1.ServiceClient
}

var _ sessionmanager.Service = (*MockServiceStruct)(nil)

func NewMockService(trust trustmappingv1.ServiceClient) sessionmanager.Service {
	return &MockServiceStruct{
		Client: trust,
	}
}

func (c *MockServiceStruct) TrustMapping() trustmappingv1.ServiceClient {
	return c.Client
}

func (c *MockServiceStruct) Close() error {
	return nil
}
