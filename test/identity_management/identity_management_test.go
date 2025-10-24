package identity_management_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/grpc/catalog"
)

var (
	binPath string
)

func init() {
	_, filename, _, _ := runtime.Caller(0) //nolint: dogsled
	baseDir := filepath.Dir(filename)

	binPath = filepath.Join(baseDir, "../../identity-management-plugins/bin/scim")
}

func IdentityManagementPlugin(t *testing.T) *plugincatalog.Catalog {
	t.Helper()

	cat, err := catalog.New(t.Context(), config.Config{
		Plugins: []plugincatalog.PluginConfig{
			{
				Name:              "IDENTITY_MANAGEMENT",
				Type:              idmangv1.Type,
				Path:              binPath,
				LogLevel:          "debug",
				YamlConfiguration: setupYamlConfig(t),
			},
		},
	})
	assert.NoError(t, err)

	return cat
}

func setupYamlConfig(t *testing.T) string {
	t.Helper()

	cfg := struct {
		CredentialFile string
	}{
		CredentialFile: "../../env/secret/identity-management.json",
	}

	bytes, _ := yaml.Marshal(cfg)

	return string(bytes)
}

func TestCreateNotificationManager(t *testing.T) {
	catalog := IdentityManagementPlugin(t)
	defer catalog.Close()

	idmang := catalog.LookupByTypeAndName(idmangv1.Type, "IDENTITY_MANAGEMENT")
	assert.NotNil(t, idmang)

	client := idmangv1.NewIdentityManagementServiceClient(idmang.ClientConnection())
	_, err := client.GetUsersForGroup(t.Context(),
		&idmangv1.GetUsersForGroupRequest{GroupId: "Test"})
	assert.NoError(t, err)
}
