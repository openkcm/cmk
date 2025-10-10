package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/repo/sql"
	tmdb "github.com/openkcm/cmk/tenant-manager/internal/db"
)

func (f *CommandFactory) NewCreateGroupsCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-default-groups",
		Short: "Create a group for tenant. Usage: tm add-default-groups -i [tenant id]",
		Long:  "Create a group for tenant. Usage: tm add-default-groups --id [tenant id]",
		Args:  cobra.ExactArgs(0),

		//nolint:contextcheck
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				cmd.Println("Tenant id is required")

				return nil
			}

			ctx := cmd.Context()
			r := sql.NewRepository(f.dbCon)

			tenant := FindTenant(ctx, cmd, id, f.r)
			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return nil
			}

			err := tmdb.CreateDefaultGroups(cmd.Context(), tenant, r)
			if err != nil {
				if errors.Is(err, tmdb.ErrOnboardingInProgress) {
					cmd.Printf("Default groups for tenant already exists")
				} else {
					cmd.Printf("Failed to create groups: %v\n", err)
				}

				return nil
			}

			cmd.Printf("Default groups created for tenant with id %s\n", id)

			return nil
		},
	}

	cmd.SetContext(ctx)

	return cmd
}
