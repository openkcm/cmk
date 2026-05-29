package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/config"
	cliUtils "github.com/openkcm/cmk/utils/cli"
)

func NewRootCmd(ctx context.Context, cfg *config.Config) *cobra.Command {
	return cliUtils.NewRootCmdWithInfinitySleep(
		ctx,
		cfg,
		"task",
		"Async Task CLI",
		"CLI tool to manage and invoke CMK asynchronous tasks.",
	)
}
