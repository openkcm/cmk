package cli

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

func FindTenant(ctx context.Context, cmd *cobra.Command, id string, r repo.Repo) *model.Tenant {
	query := repo.NewQuery()

	tenant := &model.Tenant{ID: id}

	_, err := r.First(ctx, tenant, *query)
	if err != nil {
		cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)
		return nil
	}

	return tenant
}

func FormatTenant(tenant *model.Tenant, cmd *cobra.Command) error {
	out, err := json.MarshalIndent(tenant, "", "  ")
	if err != nil {
		return err
	}

	cmd.Println(string(out))

	return nil
}
