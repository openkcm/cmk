package testutils

import (
	"path/filepath"
	"runtime"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	certificate_issuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	keystoremanv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
)

const (
	pluginDirName = "testplugins"
	pluginBin     = "testpluginbinary"
)

type MockPlugin struct {
	name              string
	dir               string
	typ               string
	tags              []string
	yamlConfiguration string
}

var KeyStorePlugin = MockPlugin{
	typ:  keystoreopv1.Type,
	name: "TEST",
	dir:  "keystoreop",
	tags: []string{"hyok", "default_keystore"},
}

var KeystoreProviderPlugin = MockPlugin{
	typ:  keystoremanv1.Type,
	name: "TEST",
	dir:  "keystoreman",
	tags: []string{"keystore_provider"},
}

var SystemInfo = MockPlugin{
	typ:  systeminformationv1.Type,
	name: "SYSINFO",
	dir:  "systeminformation",
}

var CertIssuer = MockPlugin{
	typ:  certificate_issuerv1.Type,
	name: "CERT_ISSUER",
	dir:  "certificateissuer",
}

var Notification = MockPlugin{
	typ:  notificationv1.Type,
	name: "NOTIFICATION",
	dir:  "notification",
}

var ValidKeystoreAccountInfo = map[string]string{
	"AccountID": "111122223333",
	"UserID":    "123456789012",
}

func GetPluginDir(dir string) string {
	_, filename, _, _ := runtime.Caller(0) //nolint: dogsled
	pluginsPath := filepath.Join(filepath.Dir(filename), pluginDirName)

	return filepath.Join(pluginsPath, dir, pluginBin)
}

func SetupMockPlugins(mocks ...MockPlugin) []plugincatalog.PluginConfig {
	plugins := make([]plugincatalog.PluginConfig, 0, len(mocks))

	for _, mock := range mocks {
		plugins = append(plugins, plugincatalog.PluginConfig{
			Name:              mock.name,
			Type:              mock.typ,
			Path:              GetPluginDir(mock.dir),
			LogLevel:          "debug",
			Tags:              mock.tags,
			YamlConfiguration: mock.yamlConfiguration,
		})
	}

	return plugins
}
