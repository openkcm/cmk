package clients

import (
	"errors"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry"
	sessionmanager "github.com/openkcm/cmk/internal/clients/session-manager"
)

// MockFactory is a mock implementation
// that can be used for testing purposes.
// sonarignore
type MockFactory struct {
	RegistryService       registry.Service
	SessionManagerService sessionmanager.Service
}

var _ clients.Factory = (*MockFactory)(nil)

func NewMockFactory(registryService registry.Service, sessionManager sessionmanager.Service) clients.Factory {
	return &MockFactory{
		RegistryService:       registryService,
		SessionManagerService: sessionManager,
	}
}

func (f *MockFactory) Registry() registry.Service {
	return f.RegistryService
}

func (f *MockFactory) SessionManager() sessionmanager.Service {
	return f.SessionManagerService
}

func (f *MockFactory) Close() error {
	var errs []error

	if f.RegistryService != nil {
		err := f.RegistryService.Close()
		errs = append(errs, err)
	}

	if f.SessionManagerService != nil {
		err := f.SessionManagerService.Close()
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
