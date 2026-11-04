package main

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/cmd/cmkd/commands"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

var errShutdownTimeout = errors.New("service did not shutdown within timeout")

func TestCommands(t *testing.T) {
	tests := []struct {
		name                    string
		commandFunc             func() *cobra.Command
		expectedUse             string
		doesNotHaveStatusServer bool
	}{
		{
			name:        "api-server",
			commandFunc: commands.NewAPIServer,
			expectedUse: "api-server",
		},
		{
			name:        "task-scheduler",
			commandFunc: commands.NewTaskScheduler,
			expectedUse: "task-scheduler",
		},
		{
			name:        "task-worker",
			commandFunc: commands.NewTaskWorker,
			expectedUse: "task-worker",
		},
		{
			name:        "tenant-manager",
			commandFunc: commands.NewTenantManager,
			expectedUse: "tenant-manager",
		},
		{
			name:        "event-reconciler",
			commandFunc: commands.NewEventReconciler,
			expectedUse: "event-reconciler",
		},
		{
			name:                    "db-migrator",
			commandFunc:             commands.NewDBMigrator,
			expectedUse:             "db-migrator",
			doesNotHaveStatusServer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.commandFunc()

			assert.Equal(t, tt.expectedUse, cmd.Use)
			assert.NotEmpty(t, cmd.Short)
			assert.NotEmpty(t, cmd.Long)
			require.NotNil(t, cmd.RunE, "Command should have RunE function")

			if tt.doesNotHaveStatusServer {
				testutils.CreateTestConfigFile(t)
				err := cmd.RunE(cmd, []string{})
				require.NoError(t, err)
				return
			}

			cfg := testutils.CreateTestConfigFile(t)

			// The status server reports liveness as soon as it starts, which happens
			// before the command finishes initializing (plugins, DB, AMQP, operator).
			// If the interrupt lands in that window the context is cancelled mid-init
			// and the command exits with an error instead of shutting down gracefully.
			// There is no in-process "fully started" signal to wait on, so we retry:
			// a transient startup race clears within a few attempts, while a genuine
			// failure fails every attempt and still surfaces on the last one.
			const maxAttempts = 3
			var lastErr error

			for attempt := 1; attempt <= maxAttempts; attempt++ {
				lastErr = runUntilGracefulShutdown(t, tt.commandFunc(), cfg)
				if lastErr == nil {
					break
				}
				t.Logf("attempt %d/%d did not shut down gracefully: %v", attempt, maxAttempts, lastErr)

				// The previous command's status server shuts down asynchronously; wait
				// for its port to be released before reusing it, otherwise the next
				// attempt's rebind fails and status.Serve SIGTERMs the whole process.
				waitForPortFree(t, cfg.Status.Address)
			}

			assert.NoError(t, lastErr, "Service should shut down gracefully without error")
		})
	}
}

// runUntilGracefulShutdown starts a status-server command, waits for it to come
// up, sends an interrupt, and returns the command's exit error (nil on graceful
// shutdown).
func runUntilGracefulShutdown(t *testing.T, cmd *cobra.Command, cfg *config.Config) error {
	t.Helper()

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
		return err
	case <-time.After(10 * time.Second):
		return errShutdownTimeout
	}
}

// waitForPortFree blocks until nothing is accepting connections on addr, i.e. the
// previous status server has fully released the port.
func waitForPortFree(t *testing.T, addr string) {
	t.Helper()

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return true
		}
		conn.Close()
		return false
	}, 10*time.Second, 100*time.Millisecond, "status server port %s was never released", addr)
}
