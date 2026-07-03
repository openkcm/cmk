package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/cmd/cmkd/commands"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	root := &cobra.Command{
		Use:   "cmkd",
		Short: "CMK daemon - unified CLI for all CMK services",
		Long:  `CMK daemon provides a single entry point to run all CMK services`,
	}

	root.AddCommand(commands.NewAPIServer())
	root.AddCommand(commands.NewTaskScheduler())
	root.AddCommand(commands.NewTaskWorker())
	root.AddCommand(commands.NewDBMigrator())
	root.AddCommand(commands.NewTenantManager())
	root.AddCommand(commands.NewEventReconciler())

	return root.Execute()
}
