package cmd

import (
	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
)

type CommandFactory struct {
	dbCon *multitenancy.DB
	r     repo.Repo
}

func NewCommandFactory(dbCon *multitenancy.DB) *CommandFactory {
	return &CommandFactory{
		dbCon: dbCon,
		r:     sql.NewRepository(dbCon),
	}
}
