package workflow

import (
	"strings"

	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// ToAPI converts a workflow model to an API workflow presentation.
func ToAPI(w model.Workflow) (*cmkapi.Workflow, error) {
	return &cmkapi.Workflow{
		Id:            ptr.PointTo(w.ID),
		InitiatorID:   ptr.PointTo(w.InitiatorID),
		InitiatorName: ptr.PointTo(w.InitiatorName),
		State:         ptr.PointTo(cmkapi.WorkflowState(strings.ToUpper(w.State))),
		ActionType:    cmkapi.WorkflowActionType(strings.ToUpper(w.ActionType)),
		ArtifactType:  cmkapi.WorkflowArtifactType(strings.ToUpper(w.ArtifactType)),
		ArtifactID:    w.ArtifactID,
		Parameters:    ptr.PointTo(w.Parameters),
		FailureReason: ptr.PointTo(w.FailureReason),
		Metadata: ptr.PointTo(cmkapi.WorkflowMetadata{
			CreatedAt: ptr.PointTo(w.CreatedAt.Format(transform.DefTimeFormat)),
			UpdatedAt: ptr.PointTo(w.UpdatedAt.Format(transform.DefTimeFormat)),
		}),
	}, nil
}

// FromAPI converts an API workflow presentation to a workflow model.
func FromAPI(apiWorkflow cmkapi.Workflow, userID uuid.UUID) (*model.Workflow, error) {
	if apiWorkflow.Id == nil {
		newUUID := uuid.New()
		apiWorkflow.Id = &newUUID
	}

	wf := &model.Workflow{
		ID:            *apiWorkflow.Id,
		ActionType:    strings.ToUpper(string(apiWorkflow.ActionType)),
		ArtifactType:  strings.ToUpper(string(apiWorkflow.ArtifactType)),
		ArtifactID:    apiWorkflow.ArtifactID,
		InitiatorID:   userID,
		InitiatorName: "", // TBD how to get the name
	}

	if apiWorkflow.Parameters != nil {
		wf.Parameters = *apiWorkflow.Parameters
	}

	return wf, nil
}
