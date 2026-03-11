package manager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

const providerTest = "TEST"

func TestNewManager(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dbRepo := sql.NewRepository(db)

	ps, psCfg := testutils.NewTestPlugins(testplugins.NewSystemInformation())

	cfg := &config.Config{
		Plugins: psCfg,
	}
	svcRegistry, err := cmkpluginregistry.New(t.Context(), cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(t, err)

	factory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)

	m := manager.New(t.Context(), dbRepo, cfg, factory, svcRegistry, nil, nil, nil)

	assert.NotNil(t, m)
	assert.NotNil(t, m.Keys)
	assert.NotNil(t, m.KeyVersions)
	assert.NotNil(t, m.TenantConfigs)
	assert.NotNil(t, m.Catalog)
}
