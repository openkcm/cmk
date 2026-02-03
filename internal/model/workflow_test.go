package model_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
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

func TestWorkflow_Description(t *testing.T) {
	artifactID := uuid.New()
	initiatorName := "initiator@example.com"
	keyConfigID := uuid.NewString()
	keyConfigName := "KeyConfiguration-name"

	tests := []struct {
		name                   string
		artifactType           string
		actionType             string
		artifactName           *string
		parameters             string
		parametersResourceType *string
		parametersResourceName *string
		expectedDescription    string
	}{
		{
			name:                   "SYSTEM LINK with artifact name and resource name",
			artifactType:           "SYSTEM",
			actionType:             "LINK",
			artifactName:           ptr.PointTo("Production System"),
			parameters:             keyConfigID,
			parametersResourceType: ptr.PointTo("KEY_CONFIGURATION"),
			parametersResourceName: ptr.PointTo(keyConfigName),
			expectedDescription:    "initiator@example.com requested approval to LINK SYSTEM: 'Production System' to KEY_CONFIGURATION: 'KeyConfiguration-name'.",
		},
		{
			name:                   "SYSTEM LINK without artifact name with resource name",
			artifactType:           "SYSTEM",
			actionType:             "LINK",
			artifactName:           nil,
			parameters:             keyConfigID,
			parametersResourceType: ptr.PointTo("KEY_CONFIGURATION"),
			parametersResourceName: ptr.PointTo(keyConfigName),
			expectedDescription:    "initiator@example.com requested approval to LINK SYSTEM to KEY_CONFIGURATION: 'KeyConfiguration-name'.",
		},
		{
			name:                   "SYSTEM LINK with artifact name but no resource name",
			artifactType:           "SYSTEM",
			actionType:             "LINK",
			artifactName:           ptr.PointTo("Production System"),
			parameters:             keyConfigID,
			parametersResourceType: ptr.PointTo("KEY_CONFIGURATION"),
			parametersResourceName: nil,
			expectedDescription:    "initiator@example.com requested approval to LINK SYSTEM: 'Production System'.",
		},
		{
			name:                   "SYSTEM UNLINK with artifact name",
			artifactType:           "SYSTEM",
			actionType:             "UNLINK",
			artifactName:           ptr.PointTo("Production System"),
			parameters:             "",
			parametersResourceType: nil,
			parametersResourceName: nil,
			expectedDescription:    "initiator@example.com requested approval to UNLINK SYSTEM: 'Production System'.",
		},
		{
			name:                   "SYSTEM UNLINK without artifact name",
			artifactType:           "SYSTEM",
			actionType:             "UNLINK",
			artifactName:           nil,
			parameters:             "",
			parametersResourceType: nil,
			parametersResourceName: nil,
			expectedDescription:    "initiator@example.com requested approval to UNLINK SYSTEM.",
		},
		{
			name:                   "SYSTEM SWITCH with artifact name and resource name",
			artifactType:           "SYSTEM",
			actionType:             "SWITCH",
			artifactName:           ptr.PointTo("Staging System"),
			parameters:             keyConfigID,
			parametersResourceType: ptr.PointTo("KEY_CONFIGURATION"),
			parametersResourceName: ptr.PointTo(keyConfigName),
			expectedDescription:    "initiator@example.com requested approval to SWITCH SYSTEM: 'Staging System' to KEY_CONFIGURATION: 'KeyConfiguration-name'.",
		},
		{
			name:                   "SYSTEM SWITCH without artifact name with resource name",
			artifactType:           "SYSTEM",
			actionType:             "SWITCH",
			artifactName:           nil,
			parameters:             keyConfigID,
			parametersResourceType: ptr.PointTo("KEY_CONFIGURATION"),
			parametersResourceName: ptr.PointTo(keyConfigName),
			expectedDescription:    "initiator@example.com requested approval to SWITCH SYSTEM to KEY_CONFIGURATION: 'KeyConfiguration-name'.",
		},
		{
			name:                   "KEY DELETE with artifact name",
			artifactType:           "KEY",
			actionType:             "DELETE",
			artifactName:           ptr.PointTo("Test Key"),
			parameters:             "",
			parametersResourceType: nil,
			parametersResourceName: nil,
			expectedDescription:    "initiator@example.com requested approval to DELETE KEY: 'Test Key'.",
		},
		{
			name:                   "KEY DELETE without artifact name",
			artifactType:           "KEY",
			actionType:             "DELETE",
			artifactName:           nil,
			parameters:             "",
			parametersResourceType: nil,
			parametersResourceName: nil,
			expectedDescription:    "initiator@example.com requested approval to DELETE KEY.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := model.Workflow{
				ID:                     uuid.New(),
				InitiatorName:          initiatorName,
				ActionType:             tt.actionType,
				ArtifactType:           tt.artifactType,
				ArtifactName:           tt.artifactName,
				ArtifactID:             artifactID,
				Parameters:             tt.parameters,
				ParametersResourceType: tt.parametersResourceType,
				ParametersResourceName: tt.parametersResourceName,
			}

			description := workflow.Description()

			assert.Equal(t, tt.expectedDescription, description)
		})
	}
}
