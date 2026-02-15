package manager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

const providerTest = "TEST"

func TestNewManager(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dbRepo := sql.NewRepository(db)
	svcRegistry := &cmkplugincatalog.Registry{}

	cfg := &config.Config{}

	factory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)

	m := manager.New(t.Context(), dbRepo, cfg, factory, svcRegistry, nil, nil, nil)

	assert.NotNil(t, m)
	assert.NotNil(t, m.Keys)
	assert.NotNil(t, m.KeyVersions)
	assert.NotNil(t, m.TenantConfigs)
	assert.NotNil(t, m.Catalog)
}
