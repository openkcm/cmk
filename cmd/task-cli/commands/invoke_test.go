package commands_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
)

func TestInvokeCmd_Success_NoTenants(t *testing.T) {
	ctx := context.Background()
	client := async.MockClient{}

	cmd := commands.NewInvokeCmd(ctx, &client)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", config.TypeCertificateTask})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task cert:rotate enqueued with ID: mock-task-id")
}

func TestInvokeCmd_Success_WithTenants(t *testing.T) {
	ctx := context.Background()
	client := async.MockClient{}

	cmd := commands.NewInvokeCmd(ctx, &client)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", config.TypeCertificateTask})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task cert:rotate enqueued with ID: mock-task-id")
}

func TestInvokeCmd_UnknownTask(t *testing.T) {
	ctx := context.Background()
	client := async.MockClient{}

	cmd := commands.NewInvokeCmd(ctx, &client)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", "unknown-task"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Unknown task name or not supported: unknown-task")
}
