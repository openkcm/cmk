package notification_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	integrationutils "github.com/openkcm/cmk/test/integration_utils"
)

var ansPath string

func init() {
	_, filename, _, _ := runtime.Caller(0) //nolint: dogsled
	baseDir := filepath.Dir(filename)

	ansPath = filepath.Join(baseDir, "../../notification-plugins/bin/notification")
}

func NotificationPlugin(t *testing.T) *plugincatalog.Catalog {
	t.Helper()
	plugins, err := catalog.New(t.Context(), config.Config{
		Plugins: []plugincatalog.PluginConfig{
			integrationutils.NotificationPlugin(t),
		},
	})
	assert.NoError(t, err)

	return plugins
}

func TestCreateNotificationManager(t *testing.T) {
	pluginCatalog := NotificationPlugin(t)
	defer pluginCatalog.Close()

	m := manager.NewNotificationManager(t.Context(), pluginCatalog)

	err := m.CreateNotification(t.Context(), manager.ANSNotification{
		Recipients: []string{"TestRecipient"},
		Subject:    "Test Notification",
		Body:       "This was a test notification",
	})
	assert.NoError(t, err)
}
