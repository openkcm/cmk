package notifier_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/notifier"
	"github.tools.sap/kms/cmk/internal/notifier/workflow"
)

func TestNewNotifier(t *testing.T) {
	n, err := notifier.New()
	assert.NoError(t, err)

	assert.NotNil(t, n, "Notifier should not be nil")
	assert.NotNil(t, n.Workflow(), "Workflow creator should not be nil")
}

func TestNotifier_Workflow(t *testing.T) {
	n, err := notifier.New()
	assert.NoError(t, err)

	workflowCreator := n.Workflow()

	assert.NotNil(t, workflowCreator, "Workflow creator should not be nil")
	assert.IsType(t, &workflow.Creator{}, workflowCreator, "Should return workflow.Creator type")
}
