package plugins

import (
	"github.com/openkcm/plugin-sdk/pkg/catalog"
  
  "github.com/openkcm/cmk/internal/plugins/identity-management/scim"

	certificateissuernoop "github.com/openkcm/cmk/internal/plugins/certificate-issuer/noop"
	identitymanagementnoop "github.com/openkcm/cmk/internal/plugins/identity-management/noop"
	keymanagementnoop "github.com/openkcm/cmk/internal/plugins/key-management/noop"
	keystoremanagementnoop "github.com/openkcm/cmk/internal/plugins/keystore-management/noop"
	notificationnoop "github.com/openkcm/cmk/internal/plugins/notification/noop"
	systeminformationnoop "github.com/openkcm/cmk/internal/plugins/system-information/noop"
)

func RegisterAllBuiltInPlugins(registry catalog.BuiltInPluginRegistry) {
	identitymanagementnoop.Register(registry)
	notificationnoop.Register(registry)
	systeminformationnoop.Register(registry)
	certificateissuernoop.Register(registry)
	keystoremanagementnoop.Register(registry)
	keymanagementnoop.Register(registry)
  
  scim.Register(registry)
}
	
