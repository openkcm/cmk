package plugins

import (
	"github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim"
)

func RegisterAllBuiltInPlugins(registry catalog.BuiltInPluginRegistry) {
	scim.Register(registry)
}
