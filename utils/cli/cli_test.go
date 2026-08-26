package cli_test

import (
	"bytes"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/utils/cli"
)

func TestRootCmdHandlesSignalGracefully(t *testing.T) {
	cmd := cli.NewSleep(&config.Config{})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

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
