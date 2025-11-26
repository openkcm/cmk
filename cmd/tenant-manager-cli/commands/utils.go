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
