package commands

import (
	"context"

	"github.com/spf13/cobra"

	cliUtils "github.com/openkcm/cmk/utils/cli"
)

func NewRootCmd(ctx context.Context) *cobra.Command {
	return cliUtils.NewRootCmdWithInfinitySleep(
		ctx,
		"task",
		"Async Task CLI",
		"CLI tool to manage and invoke CMK asynchronous tasks.",
	)
}
