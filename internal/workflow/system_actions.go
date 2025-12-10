package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/model"
)

type SystemActions interface {
	PatchSystemLinkByID(ctx context.Context, systemID uuid.UUID, patchSystem cmkapi.SystemPatch) (*model.System, error)
	DeleteSystemLinkByID(ctx context.Context, systemID uuid.UUID) error
}

func (l *Lifecycle) systemLinkOrSwitch(ctx context.Context) error {
	systemID := l.Workflow.ArtifactID

	keyConfigurationID, err := uuid.Parse(l.Workflow.Parameters)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	_, err = l.SystemActions.PatchSystemLinkByID(ctx, systemID, cmkapi.SystemPatch{KeyConfigurationID: keyConfigurationID})
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}

func (l *Lifecycle) systemUnlink(ctx context.Context) error {
	systemID := l.Workflow.ArtifactID

	err := l.SystemActions.DeleteSystemLinkByID(ctx, systemID)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}
