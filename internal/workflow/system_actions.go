package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
)

type SystemActions interface {
	LinkSystemAction(ctx context.Context, systemID uuid.UUID, patchSystem cmkapi.SystemPatch) (*model.System, error)
	UnlinkSystemAction(ctx context.Context, systemID uuid.UUID, trigger string) error
}

func (l *Lifecycle) systemLinkOrSwitch(ctx context.Context) error {
	systemID := l.Workflow.ArtifactID

	keyConfigurationID, err := uuid.Parse(l.Workflow.Parameters)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	_, err = l.SystemActions.LinkSystemAction(ctx, systemID, cmkapi.SystemPatch{KeyConfigurationID: keyConfigurationID})
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}

func (l *Lifecycle) systemUnlink(ctx context.Context) error {
	systemID := l.Workflow.ArtifactID

	err := l.SystemActions.UnlinkSystemAction(ctx, systemID, "")
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}
