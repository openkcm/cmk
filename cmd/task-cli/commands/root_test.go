// go
package commands_test

import (
	"bytes"
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
)

func TestRootCmdProvidesSleepFlag(t *testing.T) {
	ctx := context.Background()
	cmd := commands.NewRootCmd(ctx)

	assert.NotNil(t, cmd.PersistentFlags().Lookup("sleep"))
}

func TestRootCmdSleepModePrintsStartAndShutdown(t *testing.T) {
	ctx := context.Background()
	cmd := commands.NewRootCmd(ctx)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--sleep"})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	time.Sleep(10 * time.Millisecond)
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	err := <-done
	assert.NoError(t, err)
	got := out.String()
	assert.Contains(t, got, "Pod running...")
	assert.Contains(t, got, "Shutting down gracefully...")
}
