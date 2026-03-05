package testutils

import (
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

type PluginConfig struct {
	Tags              []string
	YamlConfiguration string
}

// map[pluginType]tags
var pluginTags = map[string]PluginConfig{
	servicewrapper.IdentityManagementType: {},
	servicewrapper.CertificateIssuerType:  {},
	servicewrapper.NotificationType:       {},
	servicewrapper.SystemInformationType:  {},
	servicewrapper.KeystoreManagementType: {
		Tags: []string{"keystore_provider"},
	},
	servicewrapper.KeyManagementType: {
		Tags: []string{"hyok", "default_keystore"},
	},
}

var ValidKeystoreAccountInfo = map[string]string{
	"AccountID": "111122223333",
	"UserID":    "123456789012",
}

func NewTestPlugins(plugins ...plugincatalog.BuiltInPlugin) (
	[]plugincatalog.BuiltInPlugin,
	[]plugincatalog.PluginConfig,
) {
	pluginCfgs := make([]plugincatalog.PluginConfig, 0, len(plugins))
	for _, p := range plugins {
		pluginCfg := plugincatalog.PluginConfig{
			Name: p.Name(),
			Type: p.Type(),
			Tags: p.Tags(),
		}

		values, ok := pluginTags[p.Type()]
		if ok {
			pluginCfg.Tags = values.Tags
		}

		pluginCfgs = append(pluginCfgs, pluginCfg)
	}
	return plugins, pluginCfgs
}
