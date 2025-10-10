package cmd

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
)

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

			tenant := FindTenant(cmd.Context(), cmd, id, f.r)
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

	cmd.SetContext(ctx)

	return cmd
}
