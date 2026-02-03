package commands

import (
	"context"

	"github.com/spf13/cobra"

	cliUtils "github.com/openkcm/cmk/utils/cli"
)

func (f *CommandFactory) NewRootCmd(ctx context.Context) *cobra.Command {
	return cliUtils.NewRootCmdWithInfinitySleep(
		ctx,
		"tm",
		"Tenant Manager CLI Application",
		"Tenant Manager is a simple CLI tool to manage tenants, supporting: creating tenant, "+
			"creating tenant with groups, "+
			"creating groups, "+
			"updating of region and status field on a tenant entity in public table, "+
			"updating of group names, "+
			"changing any field value in any table of a tenant schema.",
	)
}
