package commands

import (
	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
)

type CommandFactory struct {
	dbCon *multitenancy.DB
	r     repo.Repo
	gm    *manager.GroupManager
	tm    *manager.TenantManager
}

func NewCommandFactory(dbCon *multitenancy.DB, catalog *plugincatalog.Catalog) *CommandFactory {
	repository := sql.NewRepository(dbCon)

	return &CommandFactory{
		dbCon: dbCon,
		r:     repository,
		gm:    manager.NewGroupManager(repository, catalog),
		tm:    manager.NewTenantManager(repository),
	}
}
