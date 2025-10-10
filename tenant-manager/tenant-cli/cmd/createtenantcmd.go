package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/model"
	tmdb "github.com/openkcm/cmk/tenant-manager/internal/db"
	"github.com/openkcm/cmk/utils/base62"
)

func (f *CommandFactory) NewCreateTenantCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new tenant. Usage: tm create -i [tenant id] -r [tenant region] -s [tenant status]",
		Long:  "Create a new tenant. Usage: tm create -id [tenant id] -region [tenant region] -status [tenant status]",
		Args:  cobra.ExactArgs(0),

		//nolint:contextcheck
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			region, _ := cmd.Flags().GetString("region")
			status, _ := cmd.Flags().GetString("status")

			if id == "" || region == "" || status == "" {
				cmd.Println("Tenant id, is required")
				return ErrTenantIDRequired
			}

			if status == "" {
				cmd.Println("Tenant status is required")
				return ErrTenantStatusRequired
			}

			if region == "" {
				cmd.Println("Tenant region is required")
				return ErrTenantRegionRequired
			}

			encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
			if err != nil {
				cmd.Printf("Failed to encode schema name: %v\n", err)
				return err
			}

			tenant := &model.Tenant{
				ID:     id,
				Region: region,
				Status: model.TenantStatus(status),
				TenantModel: multitenancy.TenantModel{
					DomainURL:  encodedSchemaName,
					SchemaName: encodedSchemaName,
				},
			}

			err = tmdb.CreateSchema(cmd.Context(), f.dbCon, tenant)
			if err != nil {
				if errors.Is(err, tmdb.ErrOnboardingInProgress) {
					cmd.Printf("Tenant with ID: %s already exists", tenant.ID)
				} else {
					cmd.Printf("Failed to create tenant schema: %v\n", err)
				}
			}

			cmd.Printf("Tenant schema created: %s\n", encodedSchemaName)

			return nil
		},
	}

	cmd.SetContext(ctx)

	return cmd
}
