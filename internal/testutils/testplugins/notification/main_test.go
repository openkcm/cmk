package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	tp "github.com/openkcm/cmk/internal/testutils/testplugins/notification"
)

func TestConfigureReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.Configure(t.Context(), &configv1.ConfigureRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.SendNotification(t.Context(), &notificationv1.SendNotificationRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewCreatesTestPluginInstance(t *testing.T) {
	plugin := tp.New()
	assert.NotNil(t, plugin)
	assert.Implements(t, (*notificationv1.NotificationServiceServer)(nil), plugin)
}
