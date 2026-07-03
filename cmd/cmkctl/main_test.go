package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/cmd/cmkctl/commands"
	taskCLI "github.com/openkcm/cmk/cmd/cmkctl/commands/taskcli"
	tenantManagerCLI "github.com/openkcm/cmk/cmd/cmkctl/commands/tenantmanagercli"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestCommandsWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		commandFunc func() *cobra.Command
		expectedUse string
	}{
		{
			name:        "tenant-manager-cli",
			commandFunc: tenantManagerCLI.NewTenantManagerCLI,
			expectedUse: "tenant",
		},
		{
			name:        "task-cli",
			commandFunc: taskCLI.NewTaskCLI,
			expectedUse: "task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.commandFunc()

			assert.Equal(t, tt.expectedUse, cmd.Use)
			assert.NotEmpty(t, cmd.Short)

			_ = testutils.CreateTestConfigFile(t)

			_ = cmd.PersistentPreRunE(cmd, []string{})
		})
	}

	t.Run("sleep", func(t *testing.T) {
		cmd := commands.NewSleep()
		cfg := testutils.CreateTestConfigFile(t)

		errChan := make(chan error, 1)

		go func() {
			cmd.SetContext(context.Background())
			errChan <- cmd.RunE(cmd, []string{})
		}()

		// If status server gives back 200, service has started
		testutils.WaitForServer(t, cfg.Status.Address)

		// Send interrupt signal to trigger graceful shutdown
		p, err := os.FindProcess(os.Getpid())
		require.NoError(t, err, "failed to get process")
		err = p.Signal(os.Interrupt)
		require.NoError(t, err, "failed to send interrupt signal")

		// Wait for the service to exit
		select {
		case err := <-errChan:
			assert.NoError(t, err, "Service should shut down gracefully without error")
		case <-time.After(5 * time.Second):
			assert.Fail(t, "Service did not shutdown within timeout")
		}
	})
}
