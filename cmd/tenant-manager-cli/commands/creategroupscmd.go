package commands

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/manager"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// NewCreateGroupsCmd creates a Cobra command that creates tenant groups
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

			tenant, err := f.tm.GetTenantByID(ctx, id)
			if err != nil {
				cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

				return err
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return ErrTenantNotFound
			}

			groupCtx := cmkcontext.CreateTenantContext(ctx, tenant.ID)

			err = f.gm.CreateDefaultGroups(groupCtx)
			if err != nil {
				if errors.Is(err, manager.ErrOnboardingInProgress) {
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

	var id string

	cmd.SetContext(ctx)
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	return cmd
}
