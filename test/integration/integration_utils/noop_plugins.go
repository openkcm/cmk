package integrationutils

import (
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

func NoopPluginConfigs() []plugincatalog.PluginConfig {
	return []plugincatalog.PluginConfig{
		{
			Name: "noop",
			Type: servicewrapper.NotificationServiceType,
		},
		{
			Name: "noop",
			Type: servicewrapper.IdentityManagementServiceType,
		},
		{
			Name: "noop",
			Type: servicewrapper.SystemInformationServiceType,
		},
		{
			Name: "noop",
			Type: servicewrapper.CertificateIssuerServiceType,
		},
		{
			Name: "noop",
			Type: servicewrapper.KeystoreManagementType,
		},
		{
			Name: "noop",
			Type: servicewrapper.KeyManagementType,
		},
	}
}
