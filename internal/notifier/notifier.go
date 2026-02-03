package notifier

import (
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/notifier/workflow"
)

// Notifier creates different notification creators
type Notifier struct {
	workflow *workflow.Creator
}

// New creates a new notifier instance
func New(config *config.Config) (*Notifier, error) {
	workflowCreator, err := workflow.NewWorkflowCreator(config)
	if err != nil {
		return nil, err
	}

	return &Notifier{
		workflow: workflowCreator,
	}, nil
}

// Workflow returns the workflow notification creator
func (f *Notifier) Workflow() *workflow.Creator {
	return f.workflow
}
