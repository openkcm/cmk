package commands

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/utils/context"
)

// NewGetTenantCmd creates a Cobra command that gets tenant information.
func NewGetTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get tenant by id. Usage: tm get -i [tenant id]",
		Long:  "Get tenant by id. Usage: tm get --id [tenant id]",
		Args:  cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := context.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			tenant, err := GetTenant(cmd, f)
			if err != nil {
				return err
			}

			out, err := json.MarshalIndent(tenant, "", "  ")
			if err != nil {
				return err
			}

			cmd.Println(string(out))

			return nil
		},
	}

	var id string
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	return cmd
}
