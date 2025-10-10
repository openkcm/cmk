package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

func (f *CommandFactory) NewUpdateTenantCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update existing tenant. Usage: tm update -i [tenant id] (-r [tenant region]) (-s [tenant status])",
		Long: "Update existing tenant. Usage: tm update --id [tenant id] " +
			"(--region [tenant region]) (--status [tenant status])",
		Args: cobra.ExactArgs(0),

		//nolint:contextcheck
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			region, _ := cmd.Flags().GetString("region")
			status, _ := cmd.Flags().GetString("status")

			if id == "" {
				cmd.Println("Tenant id is required")
				return ErrTenantIDRequired
			}

			ctx := cmd.Context()

			tenant := FindTenant(ctx, cmd, id, f.r)

			query := repo.NewQuery()

			if status != "" {
				tenant.Status = model.TenantStatus(status)
			}

			if region != "" {
				tenant.Region = region
			}

			_, err := f.r.Patch(ctx, tenant, *query)
			if err != nil {
				cmd.PrintErrf("Failed to update tenant: %v\n", err)
				return err
			}

			cmd.Print("Tenant updated")

			return nil
		},
	}

	cmd.SetContext(ctx)

	return cmd
}
