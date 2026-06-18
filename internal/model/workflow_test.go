package model_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/enums"
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
	keyConfigID := uuid.NewString()
	keyConfigName := "KeyConfiguration-name"

	tests := []struct {
		name                   string
		artifactType           model.WorkflowArtifactType
		actionType             model.WorkflowActionType
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

	initiatorID := uuid.NewString()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := model.Workflow{
				ID:                     uuid.New(),
				InitiatorID:            initiatorID,
				ActionType:             tt.actionType,
				ArtifactType:           tt.artifactType,
				ArtifactName:           tt.artifactName,
				ArtifactID:             artifactID,
				Parameters:             tt.parameters,
				ParametersResourceType: tt.parametersResourceType,
				ParametersResourceName: tt.parametersResourceName,
			}

			idm := testplugins.NewTestIdentityManagement()
			idm.PutUser(identitymanagement.User{ID: initiatorID, Name: "initiator@example.com"})

			ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)

			description, err := workflow.Description(ctx, idm)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedDescription, description)
		})
	}
}

func TestWorkflowState_Valid(t *testing.T) {
	assert.True(t, model.WorkflowStateInitial.Valid())
	assert.False(t, model.WorkflowState("").Valid())
	assert.False(t, model.WorkflowState("BOGUS").Valid())
}

func TestWorkflowState_Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v, err := model.WorkflowStateInitial.Value()
		assert.NoError(t, err)
		assert.Equal(t, "INITIAL", v)
	})

	t.Run("empty becomes NULL", func(t *testing.T) {
		v, err := model.WorkflowState("").Value()
		assert.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := model.WorkflowState("BOGUS").Value()
		assert.ErrorIs(t, err, model.ErrInvalidWorkflowState)
	})
}

func TestWorkflowState_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    model.WorkflowState
		wantErr error
	}{
		{name: "string", src: "INITIAL", want: model.WorkflowStateInitial},
		{name: "bytes", src: []byte("WAIT_APPROVAL"), want: model.WorkflowStateWaitApproval},
		{name: "nil clears", src: nil, want: model.WorkflowState("")},
		{name: "invalid", src: "BOGUS", wantErr: model.ErrInvalidWorkflowState},
		{name: "wrong type", src: 123, wantErr: enums.ErrUnexpectedScanType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s model.WorkflowState
			err := s.Scan(tt.src)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestWorkflowArtifactType_Valid(t *testing.T) {
	assert.True(t, model.WorkflowArtifactTypeKey.Valid())
	assert.False(t, model.WorkflowArtifactType("").Valid())
	assert.False(t, model.WorkflowArtifactType("BOGUS").Valid())
}

func TestWorkflowArtifactType_Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v, err := model.WorkflowArtifactTypeKey.Value()
		assert.NoError(t, err)
		assert.Equal(t, "KEY", v)
	})

	t.Run("empty becomes NULL", func(t *testing.T) {
		v, err := model.WorkflowArtifactType("").Value()
		assert.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := model.WorkflowArtifactType("BOGUS").Value()
		assert.ErrorIs(t, err, model.ErrInvalidWorkflowArtifactType)
	})
}

func TestWorkflowArtifactType_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    model.WorkflowArtifactType
		wantErr error
	}{
		{name: "string", src: "KEY", want: model.WorkflowArtifactTypeKey},
		{name: "bytes", src: []byte("SYSTEM"), want: model.WorkflowArtifactTypeSystem},
		{name: "nil clears", src: nil, want: model.WorkflowArtifactType("")},
		{name: "invalid", src: "BOGUS", wantErr: model.ErrInvalidWorkflowArtifactType},
		{name: "wrong type", src: 3.14, wantErr: enums.ErrUnexpectedScanType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a model.WorkflowArtifactType
			err := a.Scan(tt.src)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, a)
		})
	}
}

func TestWorkflowActionType_Valid(t *testing.T) {
	assert.True(t, model.WorkflowActionTypeDelete.Valid())
	assert.False(t, model.WorkflowActionType("").Valid())
	assert.False(t, model.WorkflowActionType("BOGUS").Valid())
}

func TestWorkflowActionType_Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v, err := model.WorkflowActionTypeDelete.Value()
		assert.NoError(t, err)
		assert.Equal(t, "DELETE", v)
	})

	t.Run("empty becomes NULL", func(t *testing.T) {
		v, err := model.WorkflowActionType("").Value()
		assert.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := model.WorkflowActionType("BOGUS").Value()
		assert.ErrorIs(t, err, model.ErrInvalidWorkflowActionType)
	})
}

func TestWorkflowActionType_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    model.WorkflowActionType
		wantErr error
	}{
		{name: "string", src: "DELETE", want: model.WorkflowActionTypeDelete},
		{name: "bytes", src: []byte("LINK"), want: model.WorkflowActionTypeLink},
		{name: "nil clears", src: nil, want: model.WorkflowActionType("")},
		{name: "invalid", src: "INVALID", wantErr: model.ErrInvalidWorkflowActionType},
		{name: "wrong type", src: 42, wantErr: enums.ErrUnexpectedScanType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a model.WorkflowActionType
			err := a.Scan(tt.src)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, a)
		})
	}
}
