package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
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
