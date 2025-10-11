package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/cmk/cmd/apiserver"
	"github.com/openkcm/cmk/cmd/taskscheduler"
	"github.com/openkcm/cmk/cmd/taskworker"
	"github.com/openkcm/cmk/cmd/tenantmanager"
	"github.com/openkcm/cmk/cmd/tenantmanagercli"
	"github.com/openkcm/common-sdk/pkg/utils"
	"github.com/spf13/cobra"
	slogctx "github.com/veqryn/slog-context"
)

var (
	// BuildInfo will be set by the build system
	BuildInfo = "{}"

	isVersionCmd            bool
	gracefulShutdownSec     int64
	gracefulShutdownMessage string
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cmk",
		Short: "OpenKCM CMK - Customer Manager Keys",
		Long:  `OpenKCM Customer Manager Keys(CMK) is a key management service to manage encryption keys for applications and services.`,
	}

	cmd.PersistentFlags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.PersistentFlags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "version",
			Short: "CMK Version",
			RunE: func(cmd *cobra.Command, args []string) error {
				isVersionCmd = true
				value, err := utils.ExtractFromComplexValue(BuildInfo)
				if err != nil {
					return err
				}
				fmt.Println(value)
				return nil
			},
		},
		apiserver.Cmd(BuildInfo),
		taskscheduler.Cmd(BuildInfo),
		taskworker.Cmd(BuildInfo),
		tenantmanager.Cmd(BuildInfo),
		tenantmanagercli.Cmd(BuildInfo),
	)

	return cmd
}

func main() {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	err := rootCmd().ExecuteContext(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to start the application", "error", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// graceful shutdown so running goroutines may finish
	if !isVersionCmd {
		_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(gracefulShutdownMessage, gracefulShutdownSec))
		time.Sleep(time.Duration(gracefulShutdownSec) * time.Second)
	}
}
