package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/utils"
	"github.com/spf13/cobra"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/tenant-manager/cli"
)

var (
	// BuildInfo will be set by the build system
	BuildInfo = "{}"

	isVersionCmd            bool
	gracefulShutdownSec     int64
	gracefulShutdownMessage string
)
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Tenant Manager CLI Version",
	RunE: func(cmd *cobra.Command, _ []string) error {
		isVersionCmd = true

		value, err := utils.ExtractFromComplexValue(BuildInfo)
		if err != nil {
			return err
		}

		slog.InfoContext(cmd.Context(), value)

		return nil
	},
}

func rootCmd() *cobra.Command {
	factory := cli.NewCommandFactory()

	return cli.InitWithCommandFactory(
		factory.NewCreateGroupsCmd(),
		factory.NewCreateTenantCmd(),
		factory.NewDeleteTenantCmd(),
		factory.NewGetTenantCmd(),
		factory.NewListTenantsCmd(),
		factory.NewUpdateTenantCmd(),
		versionCmd,
	)
}

func execute() error {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	err := rootCmd().ExecuteContext(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to start the application", "error", err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		return err
	}

	// graceful shutdown so running goroutines may finish
	if !isVersionCmd {
		_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(gracefulShutdownMessage, gracefulShutdownSec))
		time.Sleep(time.Duration(gracefulShutdownSec) * time.Second)
	}

	return nil
}

func main() {
	err := execute()
	if err != nil {
		os.Exit(1)
	}
}
