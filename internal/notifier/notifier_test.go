package notifier_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/notifier"
	"github.com/openkcm/cmk/internal/notifier/workflow"
)

var (
	testConfig = &config.Config{
		Landscape: config.Landscape{
			Name:      "Staging",
			UIBaseUrl: "https://cmk-staging.example.com",
		},
	}
)

func TestNewNotifier(t *testing.T) {
	n, err := notifier.New(testConfig)
	assert.NoError(t, err)

	assert.NotNil(t, n, "Notifier should not be nil")
	assert.NotNil(t, n.Workflow(), "Workflow creator should not be nil")
}

func TestNotifier_Workflow(t *testing.T) {
	n, err := notifier.New(testConfig)
	assert.NoError(t, err)

	workflowCreator := n.Workflow()

	assert.NotNil(t, workflowCreator, "Workflow creator should not be nil")
	assert.IsType(t, &workflow.Creator{}, workflowCreator, "Should return workflow.Creator type")
}
