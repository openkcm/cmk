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

			id, _ := cmd.Flags().GetString("id")

			if id == "" {
				cmd.Println("Tenant id is required")
				return ErrTenantIDRequired
			}

			tenant, err := f.tm.GetTenantByID(ctx, id)
			if err != nil {
				cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

				return nil
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return ErrTenantNotFound
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
