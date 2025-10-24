package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
)

type SystemActions interface {
	PatchSystemLinkByID(ctx context.Context, systemID uuid.UUID, patchSystem model.System) (*model.System, error)
	DeleteSystemLinkByID(ctx context.Context, systemID uuid.UUID) error
}

func (l *Lifecycle) systemLinkOrSwitch(ctx context.Context) error {
	systemID := l.Workflow.ArtifactID

	keyConfigurationID, err := uuid.Parse(l.Workflow.Parameters)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	_, err = l.SystemActions.PatchSystemLinkByID(ctx, systemID, model.System{KeyConfigurationID: &keyConfigurationID})
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
