package identity_management_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	"github.com/openkcm/cmk/internal/config"
	cmkplugincatalog "github.com/openkcm/cmk/internal/grpc/catalog"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

func IdentityManagementPlugin(t *testing.T) *cmkplugincatalog.Registry {
	t.Helper()

	cat, err := cmkplugincatalog.New(t.Context(), &config.Config{
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

	idmang := catalog.LookupByTypeAndName(idmangv1.Type, "IDENTITY_MANAGEMENT")
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
