package commands

import (
	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/utils/context"
)

// NewListTenantsCmd creates a Cobra command that gets tenant list.
func NewListTenantsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tenants. Usage: tm list",
		Long:  "List all tenants. Usage: tm list",

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := context.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			var tenants []model.Tenant

			err := f.r.List(
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
