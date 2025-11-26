package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// NewListTenantsCmd creates a Cobra command that gets tenant list.
func (f *CommandFactory) NewListTenantsCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tenants. Usage: tm list",
		Long:  "List all tenants. Usage: tm list",

		//nolint:contextcheck
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			var tenants []model.Tenant

			_, err := f.r.List(
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

	cmd.SetContext(ctx)

	return cmd
}
