package testutils

import (
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

// ValidKeystoreAccountInfo is test account data used by the keystore operator.
var ValidKeystoreAccountInfo = testplugins.ValidKeystoreAccountInfo

// NewTestPlugins returns a serviceapi.Registry pre-configured with default test
// service implementations. Pass RegistryOptions to override specific services.
func NewTestPlugins(opts ...testplugins.RegistryOption) serviceapi.Registry {
	return testplugins.NewRegistry(opts...)
}

// WithIDMPluginKC returns a KeyConfigOpt that registers the key configuration's
// CreatorID in the given IDM plugin so GetUser lookups succeed.
func WithIDMPluginKC(idm *testplugins.TestIdentityManagement) KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		idm.PutUser(identitymanagement.User{ID: kc.CreatorID})
	}
}
