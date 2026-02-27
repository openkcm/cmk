package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/utils/ptr"
)

type KeyConfigurationActions interface {
	DeleteKeyConfigurationByID(ctx context.Context, keyConfigID uuid.UUID) error
}

func (l *Lifecycle) deleteKeyConfiguration(ctx context.Context) error {
	err := l.KeyConfigurationActions.DeleteKeyConfigurationByID(ctx, l.Workflow.ArtifactID)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}

func (l *Lifecycle) updatePrimaryKey(ctx context.Context) error {
	keyID, err := uuid.Parse(l.Workflow.Parameters)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	_, err = l.KeyActions.UpdateKey(ctx, keyID, cmkapi.KeyPatch{IsPrimary: ptr.PointTo(true)})
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}
