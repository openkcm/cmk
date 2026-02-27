package identity_management_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	"github.com/openkcm/cmk/internal/config"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
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

	plugins := catalog.LookupByType(idmangv1.Type)
	if len(plugins) == 0 {
		t.Fatalf("catalog returned no identity management plugins")
	}
	if len(plugins) > 1 {
		t.Fatalf("catalog returned multiple identity management plugins")
	}
	idmang := plugins[0]
	assert.NotNil(t, idmang)

	client := idmangv1.NewIdentityManagementServiceClient(idmang.ClientConnection())

	respUsers, err := client.GetUsersForGroup(t.Context(),
		&idmangv1.GetUsersForGroupRequest{GroupId: "Test"})
	if respUsers != nil {
		slog.Info(fmt.Sprintf("UsersForGroup:%v", respUsers.GetUsers()))
	}

	assert.NoError(t, err)

	respGroups, err := client.GetGroupsForUser(t.Context(),
		&idmangv1.GetGroupsForUserRequest{UserId: "Test"})
	if respGroups != nil {
		slog.Info(fmt.Sprintf("GroupsForUser:%v", respGroups.GetGroups()))
	}

	assert.NoError(t, err)
}
