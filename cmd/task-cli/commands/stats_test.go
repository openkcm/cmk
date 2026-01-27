package commands_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
)

func TestStatsCmd_QueueInfo(t *testing.T) {
	ctx := context.Background()
	inspector := &commands.MockInspector{}

	cmd := commands.NewStatsCmd(ctx, inspector)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--queue-info"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"Queue\": \"default\"")
	assert.Contains(t, out.String(), "\"Size\": 42")
}

func TestStatsCmd_History(t *testing.T) {
	ctx := context.Background()
	inspector := &commands.MockInspector{}

	cmd := commands.NewStatsCmd(ctx, inspector)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--weekly-history"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"Processed\": 10")
	assert.Contains(t, out.String(), "\"Failed\": 2")
}

func TestStatsCmd_PendingTasks(t *testing.T) {
	ctx := context.Background()
	inspector := &commands.MockInspector{}

	cmd := commands.NewStatsCmd(ctx, inspector)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--pending-tasks"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"ID\": \"task1\"")
	assert.Contains(t, out.String(), "\"ID\": \"task2\"")
}

func TestStatsCmd_ActiveTasks(t *testing.T) {
	ctx := context.Background()
	inspector := &commands.MockInspector{}

	cmd := commands.NewStatsCmd(ctx, inspector)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--active-tasks"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"ID\": \"task3\"")
}
func TestStatsCmd_CompletedTasks(t *testing.T) {
	ctx := context.Background()
	inspector := &commands.MockInspector{}

	cmd := commands.NewStatsCmd(ctx, inspector)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--complete-tasks"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"ID\": \"task4\"")
	assert.Contains(t, out.String(), "\"ID\": \"task5\"")
}

func TestStatsCmd_ArchivedTasks(t *testing.T) {
	ctx := context.Background()
	inspector := &commands.MockInspector{}

	cmd := commands.NewStatsCmd(ctx, inspector)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--archived-tasks"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"ID\": \"task6\"")
}
