package cli_test

import (
	"bytes"
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/cli"
)

func TestRootCmdWithSleepFlagEnabledSleepsIndefinitely(t *testing.T) {
	ctx := context.Background()
	cmd := cli.NewRootCmdWithInfinitySleep(ctx, "test", "short description", "long description")

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
	assert.Contains(t, out.String(), "Pod running...")
	assert.Contains(t, out.String(), "Shutting down gracefully...")
}

func TestRootCmdWithoutSleepFlagExecutesWithoutSleeping(t *testing.T) {
	ctx := context.Background()
	cmd := cli.NewRootCmdWithInfinitySleep(ctx, "test", "short description", "long description")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.NotContains(t, out.String(), "Pod running...")
	assert.NotContains(t, out.String(), "Shutting down gracefully...")
}

func TestRootCmdHandlesSignalGracefully(t *testing.T) {
	ctx := context.Background()
	cmd := cli.NewRootCmdWithInfinitySleep(ctx, "test", "short description", "long description")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--sleep"})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	time.Sleep(10 * time.Millisecond)
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	err := <-done
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Shutting down gracefully...")
}
