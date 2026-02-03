package manager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

const providerTest = "TEST"

func TestNewManager(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dbRepo := sql.NewRepository(db)
	catalog := &plugincatalog.Catalog{}

	cfg := &config.Config{}

	factory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)

	m := manager.New(t.Context(), dbRepo, cfg, factory, catalog, nil, nil, nil)

	assert.NotNil(t, m)
	assert.NotNil(t, m.Keys)
	assert.NotNil(t, m.KeyVersions)
	assert.NotNil(t, m.TenantConfigs)
	assert.NotNil(t, m.Catalog)
}
