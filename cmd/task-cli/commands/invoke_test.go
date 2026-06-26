package commands_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/cmd/task-cli/commands"
	"github.tools.sap/kms/cmk/internal/config"
)

func TestInvokeCmd_Success_NoTenants(t *testing.T) {
	cmd := commands.SetupCommandTest(t, commands.NewInvokeCmd())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", config.TypeCertificateTask})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task cert:rotate enqueued with ID: mock-task-id")
}

func TestInvokeCmd_Success_WithTenants(t *testing.T) {
	cmd := commands.SetupCommandTest(t, commands.NewInvokeCmd())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", config.TypeCertificateTask})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task cert:rotate enqueued with ID: mock-task-id")
}

func TestInvokeCmd_UnknownTask(t *testing.T) {
	cmd := commands.SetupCommandTest(t, commands.NewInvokeCmd())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", "unknown-task"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Unknown task name or not supported: unknown-task")
}
