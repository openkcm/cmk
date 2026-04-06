package notifier

import (
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/notifier/workflow"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
)

// Notifier creates different notification creators
type Notifier struct {
	workflow *workflow.Creator
}

// New creates a new notifier instance
func New(config *config.Config, idm identitymanagement.IdentityManagement) (*Notifier, error) {
	workflowCreator, err := workflow.NewWorkflowCreator(config, idm)
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
