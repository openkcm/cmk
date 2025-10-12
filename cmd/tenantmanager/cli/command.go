package cli

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/tenant-manager/cli"
)

func Cmd() *cobra.Command {
	factory := cli.NewCommandFactory()

	return cli.InitWithCommandFactory(
		factory.NewCreateGroupsCmd(),
		factory.NewCreateTenantCmd(),
		factory.NewDeleteTenantCmd(),
		factory.NewGetTenantCmd(),
		factory.NewListTenantsCmd(),
		factory.NewUpdateTenantCmd(),
	)
}
