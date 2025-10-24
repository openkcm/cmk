package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
)

func TestWorkflowTable(t *testing.T) {
	t.Run("Should have table name workflows", func(t *testing.T) {
		expectedTableName := "workflows"

		tableName := model.Workflow{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Workflow{}.IsSharedModel())
	})
}

func TestWorkflowApproversTable(t *testing.T) {
	t.Run("Should have table name workflow_approvers", func(t *testing.T) {
		expectedTableName := "workflow_approvers"

		tableName := model.WorkflowApprover{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.WorkflowApprover{}.IsSharedModel())
	})
}
