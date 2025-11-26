package commands

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
)

// NewGetTenantCmd creates a Cobra command that gets tenant information.
func (f *CommandFactory) NewGetTenantCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get tenant by id. Usage: tm get -i [tenant id]",
		Long:  "Get tenant by id. Usage: tm get --id [tenant id]",
		Args:  cobra.ExactArgs(0),

		//nolint:contextcheck
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")

			if id == "" {
				cmd.Println("Tenant id is required")
				return ErrTenantIDRequired
			}

			ctx := cmd.Context()

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

	cmd.SetContext(ctx)

	return cmd
}
