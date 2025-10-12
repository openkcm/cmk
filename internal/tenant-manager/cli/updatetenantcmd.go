package cli

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
)

func (f *CommandFactory) NewUpdateTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update existing tenant. Usage: tm update -i [tenant id] (-r [tenant region]) (-s [tenant status])",
		Long: "Update existing tenant. Usage: tm update --id [tenant id] " +
			"(--region [tenant region]) (--status [tenant status])",
		Args: cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			id, _ := cmd.Flags().GetString("id")
			region, _ := cmd.Flags().GetString("region")
			status, _ := cmd.Flags().GetString("status")

			if id == "" {
				cmd.Println("Tenant id is required")
				return ErrTenantIDRequired
			}

			dbCon, err := f.db(ctx)
			if err != nil {
				cmd.Printf("Failed to connect to database: %v\n", err)
				return nil
			}

			r := sql.NewRepository(dbCon)

			tenant := FindTenant(ctx, cmd, id, r)

			query := repo.NewQuery()

			if status != "" {
				tenant.Status = model.TenantStatus(status)
			}

			if region != "" {
				tenant.Region = region
			}

			_, err = r.Patch(ctx, tenant, *query)
			if err != nil {
				cmd.PrintErrf("Failed to update tenant: %v\n", err)
				return err
			}

			cmd.Print("Tenant updated")

			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	cmd.Flags().StringVarP(&region, "region", "r", "", "Tenant region")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")

	return cmd
}
