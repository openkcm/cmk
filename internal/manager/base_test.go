package manager_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk-core/internal/clients"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
)

const providerTest = "TEST"

func TestNewManager(t *testing.T) {
	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{testutils.TestModel{}},
	})
	dbRepo := sql.NewRepository(db)
	catalog := &plugincatalog.Catalog{}

	cfg := &config.Config{}

	factory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)

	m := manager.New(t.Context(), dbRepo, cfg, factory, catalog, nil)

	assert.NotNil(t, m)
	assert.NotNil(t, m.Keys)
	assert.NotNil(t, m.KeyVersions)
	assert.NotNil(t, m.TenantConfigs)
	assert.NotNil(t, m.Catalog)
}
