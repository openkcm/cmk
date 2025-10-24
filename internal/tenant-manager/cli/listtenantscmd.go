package cli

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
)

func (f *CommandFactory) NewListTenantsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tenants. Usage: tm list",
		Long:  "List all tenants. Usage: tm list",

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			dbCon, err := f.db(ctx)
			if err != nil {
				cmd.Printf("Failed to connect to database: %v\n", err)
				return nil
			}

			r := sql.NewRepository(dbCon)

			var tenants []model.Tenant

			_, err = r.List(
				ctx, &model.Tenant{}, &tenants, *repo.NewQuery(),
			)
			if err != nil {
				cmd.PrintErrf("failed to get tenants")
				return err
			}

			for _, tenant := range tenants {
				err = FormatTenant(&tenant, cmd)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}
