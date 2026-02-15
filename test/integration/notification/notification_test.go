package notification_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/notifier/client"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

var ansPath string

func init() {
	_, filename, _, _ := runtime.Caller(0) //nolint: dogsled
	baseDir := filepath.Dir(filename)

	ansPath = filepath.Join(baseDir, "../../notification-plugins/bin/notification")
}

func NotificationPlugin(t *testing.T) *cmkplugincatalog.Registry {
	t.Helper()
	plugins, err := cmkplugincatalog.New(t.Context(), &config.Config{
		Plugins: []catalog.PluginConfig{
			integrationutils.NotificationPlugin(t),
		},
	})
	assert.NoError(t, err)

	return plugins
}

func TestCreateNotificationManager(t *testing.T) {
	requiredFiles := []string{
		integrationutils.NotificationEndpointsPath,
		integrationutils.NotificationUAAConfigPath,
	}

	if integrationutils.MissingFiles(t, requiredFiles) {
		return
	}

	pluginCatalog := NotificationPlugin(t)
	defer pluginCatalog.Close()

	m := client.New(t.Context(), pluginCatalog)

	err := m.CreateNotification(t.Context(), client.Data{
		Recipients: []string{"TestRecipient"},
		Subject:    "Test Notification",
		Body:       "This was a test notification",
	})
	assert.NoError(t, err)
}
