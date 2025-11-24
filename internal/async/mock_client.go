package async

import (
	"context"

	"github.com/hibiken/asynq"
)

// MockClient implements the AsyncClient interface for testing
type MockClient struct {
	CallCount int
	LastTask  *asynq.Task
	Error     error
}

func (m *MockClient) Close() error {
	return nil
}

func (m *MockClient) Enqueue(task *asynq.Task, opt ...asynq.Option) (*asynq.TaskInfo, error) {
	return m.enqueue(task, opt)
}

func (m *MockClient) EnqueueContext(_ context.Context, task *asynq.Task, opt ...asynq.Option) (*asynq.TaskInfo, error) {
	return m.enqueue(task, opt)
}

func (m *MockClient) Ping() error {
	return nil
}

func (m *MockClient) enqueue(task *asynq.Task, _ []asynq.Option) (*asynq.TaskInfo, error) {
	m.CallCount++

	m.LastTask = task
	if m.Error != nil {
		return nil, m.Error
	}

	return &asynq.TaskInfo{ID: "mock-task-id"}, nil
}
