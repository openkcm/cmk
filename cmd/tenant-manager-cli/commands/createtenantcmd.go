package commands

import (
	"errors"

	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/base62"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

var (
	ErrCreateTenant = errors.New("failed to create tenant schema")
	ErrCreateGroups = errors.New("failed to create gropus")
)

// NewCreateTenantCmd creates a Cobra command that creates tenant.
//
//nolint:funlen
func NewCreateTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new tenant. Usage: tm create -i [tenant id] -s [tenant status] -R [tenant role]",
		Long: "Create a new tenant. Usage: tm create -id [tenant id]" +
			" -status [tenant status] -role [tenant role]",
		Args: cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := cmkcontext.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			id, _ := cmd.Flags().GetString("id")
			status, _ := cmd.Flags().GetString("status")
			role, _ := cmd.Flags().GetString("role")

			encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
			if err != nil {
				cmd.Printf("Failed to encode schema name: %v\n", err)
				return err
			}

			tenant := &model.Tenant{
				ID:     id,
				Status: model.TenantStatus(status),
				Role:   model.TenantRole(role),
				TenantModel: multitenancy.TenantModel{
					DomainURL:  encodedSchemaName,
					SchemaName: encodedSchemaName,
				},
			}

			err = f.tm.CreateTenant(ctx, tenant)
			if errors.Is(err, manager.ErrOnboardingInProgress) {
				cmd.Printf("Tenant with ID: %s already exists\n", tenant.ID)
			} else if err != nil {
				cmd.Printf("Failed to create Tenant: %v\n", err)
				return errs.Wrap(ErrCreateTenant, err)
			}

			ctx = cmkcontext.CreateTenantContext(ctx, tenant.ID)

			err = f.gm.CreateDefaultGroups(ctx)
			if err != nil {
				if errors.Is(err, manager.ErrOnboardingInProgress) {
					cmd.Printf("Default groups for tenant already exists\n")
				} else if err != nil {
					cmd.Printf("Failed to create Default Gruops: %v\n", err)
					return errs.Wrap(ErrCreateGroups, err)
				}
			}

			cmd.Printf("Tenant: %s, created with schema: %s\n", id, encodedSchemaName)

			return nil
		},
	}

	var id, status, role string

	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")
	cmd.Flags().StringVarP(&role, "role", "R", "", "Tenant role")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("status")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'status' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("role")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'role' as required: %v\n", err)
	}

	return cmd
}
