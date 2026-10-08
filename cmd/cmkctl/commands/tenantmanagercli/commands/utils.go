package commands

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/model"
)

func FormatTenant(tenant *model.Tenant, cmd *cobra.Command) error {
	out, err := json.MarshalIndent(tenant, "", "  ")
	if err != nil {
		return err
	}

	cmd.Println(string(out))

	return nil
}

func GetTenant(cmd *cobra.Command, f *CommandFactory) (*model.Tenant, error) {
	id, _ := cmd.Flags().GetString("id")
	if id == "" {
		cmd.Println("Tenant id is required")
		return nil, ErrTenantIDRequired
	}

	tenant, err := f.tm.GetTenantByID(cmd.Context(), id)
	if err != nil {
		cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

		return nil, err
	}

	if tenant == nil {
		cmd.Printf("Tenant with id %s not found\n", id)

		return nil, ErrTenantNotFound
	}

	return tenant, nil
}
