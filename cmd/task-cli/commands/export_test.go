package commands

import (
	"time"

	"github.com/hibiken/asynq"
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
