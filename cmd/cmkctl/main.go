package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/cmd/cmkctl/commands"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	root := &cobra.Command{
		Use:   "cmkctl",
		Short: "CMK control - unified CLI for CMK management tools",
		Long:  `CMK control provides a single entry point for tenant management and task operations`,
	}

	root.AddCommand(commands.NewTenantManagerCLI())
	root.AddCommand(commands.NewTaskCLI())

	return root.Execute()
}
