package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
)

// NewUpdateTenantCmd creates a Cobra command that updates single tenant.
//
//nolint:funlen
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

			ctx := cmd.Context()

			tenant, err := f.tm.GetTenantByID(ctx, id)
			if err != nil {
				cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

				return nil
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return nil
			}

			query := repo.NewQuery()

			if status != "" {
				tenant.Status = model.TenantStatus(status)
			}

			if region != "" {
				tenant.Region = region
			}

			_, err = f.r.Patch(ctx, tenant, *query)
			if err != nil {
				cmd.PrintErrf("Failed to update tenant: %v\n", err)
				return err
			}

			cmd.Print("Tenant updated")

			return nil
		},
	}

	var (
		id, region, status string
	)
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	cmd.Flags().StringVarP(&region, "region", "r", "", "Tenant region")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	cmd.SetContext(ctx)

	return cmd
}
