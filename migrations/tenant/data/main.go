package tenantdatamigrations

import "github.com/pressly/goose/v3"

func GetMigrations() []*goose.Migration {
	return []*goose.Migration{
		goose.NewGoMigration(
			1,
			&goose.GoFunc{RunTx: upCopyGroupToGroups},
			&goose.GoFunc{RunTx: downCopyGroupToGroups},
		),
	}
}
