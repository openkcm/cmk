package tenantdatamigrations

import "github.com/pressly/goose/v3"

func GetMigrations() []*goose.Migration {
	return []*goose.Migration{
		goose.NewGoMigration(
			1,
			&goose.GoFunc{RunTx: upWorkflowApproverGroupTable},
			&goose.GoFunc{RunTx: downWorkflowApproverGroupTable},
		),
		goose.NewGoMigration(
			2,
			&goose.GoFunc{RunTx: upFlattenTenantConfigs},
			&goose.GoFunc{RunTx: downFlattenTenantConfigs},
		),
		goose.NewGoMigration(
			3,
			&goose.GoFunc{RunTx: upRemoveLegacyTenantConfigBlobs},
			&goose.GoFunc{RunTx: downRemoveLegacyTenantConfigBlobs},
		),
	}
}
