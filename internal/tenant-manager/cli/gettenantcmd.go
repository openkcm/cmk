package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/repo/sql"
)

func (f *CommandFactory) NewGetTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get tenant by id. Usage: tm get -i [tenant id]",
		Long:  "Get tenant by id. Usage: tm get --id [tenant id]",
		Args:  cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			id, _ := cmd.Flags().GetString("id")

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
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")

	return cmd
}
