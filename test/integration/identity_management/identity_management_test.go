package identity_management_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

func IdentityManagementPlugin(t *testing.T) *cmkpluginregistry.Registry {
	t.Helper()

	cat, err := cmkpluginregistry.New(t.Context(), &config.Config{
		Plugins: []plugincatalog.PluginConfig{integrationutils.IDMangementPlugin(t)},
	})
	assert.NoError(t, err)

	return cat
}

func TestCreateNotificationManager(t *testing.T) {
	requiredFiles := []string{
		integrationutils.IdentityManagementConfigPath,
	}
	if integrationutils.MissingFiles(t, requiredFiles) {
		return
	}

	catalog := IdentityManagementPlugin(t)
	defer catalog.Close()

	idm, err := catalog.IdentityManagement()
	assert.NoError(t, err)

	respUsers, err := idm.ListGroupUsers(t.Context(),
		&identitymanagement.ListGroupUsersRequest{GroupID: "Test"})
	if respUsers != nil {
		slog.Info(fmt.Sprintf("UsersForGroup:%v", respUsers.Users))
	}

	assert.NoError(t, err)

	respGroups, err := idm.ListUserGroups(t.Context(),
		&identitymanagement.ListUserGroupsRequest{UserID: "Test"})
	if respGroups != nil {
		slog.Info(fmt.Sprintf("GroupsForUser:%v", respGroups.Groups))
	}

	assert.NoError(t, err)
}
