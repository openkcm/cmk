package testutils

import (
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

// ValidKeystoreAccountInfo is test account data used by the keystore operator.
var ValidKeystoreAccountInfo = testplugins.ValidKeystoreAccountInfo

// NewTestPlugins returns a serviceapi.Registry pre-configured with default test
// service implementations. Pass RegistryOptions to override specific services.
func NewTestPlugins(opts ...testplugins.RegistryOption) serviceapi.Registry {
	return testplugins.NewRegistry(opts...)
}
