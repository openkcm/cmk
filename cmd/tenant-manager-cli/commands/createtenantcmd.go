package commands

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/base62"
)

// NewCreateTenantCmd creates a Cobra command that creates tenant.
//
//nolint:funlen
func (f *CommandFactory) NewCreateTenantCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new tenant. Usage: tm create -i [tenant id] -r [tenant region] -s [tenant status] -R [tenant role]",
		Long: "Create a new tenant. Usage: tm create -id [tenant id] -region [tenant region]" +
			" -status [tenant status] -role [tenant role]",
		Args: cobra.ExactArgs(0),

		//nolint:contextcheck
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			region, _ := cmd.Flags().GetString("region")
			status, _ := cmd.Flags().GetString("status")
			role, _ := cmd.Flags().GetString("role")

			encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
			if err != nil {
				cmd.Printf("Failed to encode schema name: %v\n", err)
				return err
			}

			tenant := &model.Tenant{
				ID:     id,
				Region: region,
				Status: model.TenantStatus(status),
				Role:   model.TenantRole(role),
				TenantModel: multitenancy.TenantModel{
					DomainURL:  encodedSchemaName,
					SchemaName: encodedSchemaName,
				},
			}

			err = f.tm.CreateTenant(cmd.Context(), tenant)
			if err != nil {
				if errors.Is(err, manager.ErrOnboardingInProgress) {
					cmd.Printf("Tenant with ID: %s already exists", tenant.ID)
				} else {
					cmd.Printf("Failed to create tenant schema: %v\n", err)
				}
			}

			cmd.Printf("Tenant schema created: %s\n", encodedSchemaName)

			return nil
		},
	}

	var (
		id, region, status, role string
	)

	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	cmd.Flags().StringVarP(&region, "region", "r", "", "Tenant region")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")
	cmd.Flags().StringVarP(&role, "role", "R", "", "Tenant role")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("region")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'region' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("status")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'status' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("role")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'role' as required: %v\n", err)
	}

	cmd.SetContext(ctx)

	return cmd
}
