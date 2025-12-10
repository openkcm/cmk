package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	tp "github.tools.sap/kms/cmk/internal/testutils/testplugins/identitymanagement"
)

func TestConfigureReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.Configure(t.Context(), &configv1.ConfigureRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetUsersForGroupReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.GetUsersForGroup(t.Context(), &idmangv1.GetUsersForGroupRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetGroupsForUserReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.GetGroupsForUser(t.Context(), &idmangv1.GetGroupsForUserRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewCreatesTestPluginInstance(t *testing.T) {
	plugin := tp.New()
	assert.NotNil(t, plugin)
	assert.Implements(t, (*idmangv1.IdentityManagementServiceServer)(nil), plugin)
}
