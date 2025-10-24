package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
)

func (f *CommandFactory) NewDeleteTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a new tenant. Usage: tm create -i [tenant id] -r [tenant region] -s [tenant status]",
		Long:  "Delete a new tenant. Usage: tm create -id [tenant id] -region [tenant region] -status [tenant status]",
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
			tenant := FindTenant(cmd.Context(), cmd, id, r)

			cmd.Printf("Deleting tenant. Id: %s, SchemaName: %s\n", tenant.ID, tenant.SchemaName)

			err = DropSchema(dbCon, tenant.SchemaName)
			if err != nil {
				cmd.PrintErrf("%v %v\n", ErrDeleteTenant, err)
				return err
			}

			_, err = r.Delete(ctx, &model.Tenant{ID: id}, *repo.NewQuery())
			if err != nil {
				cmd.PrintErrf("%v %v\n", ErrDeleteTenant, err)
				return err
			}

			cmd.Printf("Tenant deleted")

			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")

	return cmd
}

func DropSchema(db *multitenancy.DB, schemaName string) error {
	sql := fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)
	return db.Exec(sql).Error
}
