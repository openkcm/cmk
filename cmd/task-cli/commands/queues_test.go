package commands_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
)

func TestQueuesCmd(t *testing.T) {
	cmd := commands.SetupCommandTest(t, commands.NewQueuesCmd())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "List of asynq queues:")
	assert.Contains(t, out.String(), "- default")
	assert.Contains(t, out.String(), "- critical")
}
