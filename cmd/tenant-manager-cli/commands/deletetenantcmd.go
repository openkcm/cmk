package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/utils/context"
)

// NewDeleteTenantCmd creates a Cobra command that deletes a tenant.
//

func NewDeleteTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a new tenant. Usage: tm create -i [tenant id] -r [tenant region] -s [tenant status]",
		Long:  "Delete a new tenant. Usage: tm create -id [tenant id] -region [tenant region] -status [tenant status]",
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

				return err
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return ErrTenantNotFound
			}

			cmd.Printf("Deleting tenant. Id: %s, SchemaName: %s\n", tenant.ID, tenant.SchemaName)

			err = dropSchema(f.dbCon, tenant.SchemaName)
			if err != nil {
				cmd.PrintErrf("%v %v\n", ErrDeleteTenant, err)
				return err
			}

			_, err = f.r.Delete(ctx, &model.Tenant{ID: id}, *repo.NewQuery())
			if err != nil {
				cmd.PrintErrf("%v %v\n", ErrDeleteTenant, err)
				return err
			}

			cmd.Printf("Tenant deleted")

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

func dropSchema(db *multitenancy.DB, schemaName string) error {
	sql := fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)
	return db.Exec(sql).Error
}
