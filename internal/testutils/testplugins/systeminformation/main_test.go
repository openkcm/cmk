package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	tp "github.com/openkcm/cmk/internal/testutils/testplugins/systeminformation"
)

func TestConfigureReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.Configure(t.Context(), &configv1.ConfigureRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.Get(t.Context(), &systeminformationv1.GetRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewCreatesTestPluginInstance(t *testing.T) {
	plugin := tp.New()
	assert.NotNil(t, plugin)
	assert.Implements(t, (*systeminformationv1.SystemInformationServiceServer)(nil), plugin)
}
