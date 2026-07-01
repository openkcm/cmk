package commands_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/cmkctl/commands"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
)

type MockInspector struct{}

func (m *MockInspector) Queues() ([]string, error) {
	return []string{"default", "critical"}, nil
}

func (m *MockInspector) GetQueueInfo(queue string) (*asynq.QueueInfo, error) {
	return &asynq.QueueInfo{Queue: queue, Size: 42}, nil
}

func (m *MockInspector) History(queue string, days int) ([]*asynq.DailyStats, error) {
	return []*asynq.DailyStats{
		{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Processed: 10, Failed: 2},
		{Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Processed: 15, Failed: 1},
	}, nil
}

func (m *MockInspector) ListPendingTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return []*asynq.TaskInfo{
		{ID: "task1", Type: "typeA"},
		{ID: "task2", Type: "typeB"},
	}, nil
}

func (m *MockInspector) ListActiveTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return []*asynq.TaskInfo{
		{ID: "task3", Type: "typeC"},
	}, nil
}

func (m *MockInspector) ListCompletedTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return []*asynq.TaskInfo{
		{ID: "task4", Type: "typeD"},
		{ID: "task5", Type: "typeE"},
	}, nil
}

func (m *MockInspector) ListArchivedTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return []*asynq.TaskInfo{
		{ID: "task6", Type: "typeF"},
	}, nil
}

func SetupCommandTest(t *testing.T, cmd *cobra.Command) *cobra.Command {
	t.Helper()

	mockClient := &async.MockClient{}
	mockInspector := &MockInspector{}
	ctx := context.WithValue(t.Context(), commands.AsyncClientKey, mockClient)
	ctx = context.WithValue(ctx, commands.AsyncInspectorKey, mockInspector)
	cmd.SetContext(ctx)

	return cmd
}

func TestInvokeCmd_Success_NoTenants(t *testing.T) {
	cmd := SetupCommandTest(t, commands.NewInvokeCmd())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", config.TypeCertificateTask})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task cert:rotate enqueued with ID: mock-task-id")
}

func TestInvokeCmd_Success_WithTenants(t *testing.T) {
	cmd := SetupCommandTest(t, commands.NewInvokeCmd())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", config.TypeCertificateTask})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task cert:rotate enqueued with ID: mock-task-id")
}

func TestInvokeCmd_UnknownTask(t *testing.T) {
	cmd := SetupCommandTest(t, commands.NewInvokeCmd())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"--task", "unknown-task"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Unknown task name or not supported: unknown-task")
}

func TestQueuesCmd(t *testing.T) {
	cmd := SetupCommandTest(t, commands.NewQueuesCmd())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "List of asynq queues:")
	assert.Contains(t, out.String(), "- default")
	assert.Contains(t, out.String(), "- critical")
}

func TestStatsCmd_QueueInfo(t *testing.T) {
	cmd := SetupCommandTest(t, commands.NewStatsCmd())

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
	cmd := SetupCommandTest(t, commands.NewStatsCmd())

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
	cmd := SetupCommandTest(t, commands.NewStatsCmd())

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
	cmd := SetupCommandTest(t, commands.NewStatsCmd())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--active-tasks"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"ID\": \"task3\"")
}

func TestStatsCmd_CompletedTasks(t *testing.T) {
	cmd := SetupCommandTest(t, commands.NewStatsCmd())

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
	cmd := SetupCommandTest(t, commands.NewStatsCmd())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--queue", "default", "--archived-tasks"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "\"ID\": \"task6\"")
}
