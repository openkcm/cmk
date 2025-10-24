package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

type KeyActions interface {
	UpdateKey(
		ctx context.Context,
		keyID uuid.UUID,
		keyPatch cmkapi.KeyPatch,
	) (*model.Key, error)
	Delete(ctx context.Context, keyID uuid.UUID) error
	Get(ctx context.Context, keyID uuid.UUID) (*model.Key, error)
}

func (l *Lifecycle) updateKeyState(ctx context.Context) error {
	keyID := l.Workflow.ArtifactID

	dbKey, err := l.KeyActions.Get(ctx, keyID)
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	switch l.Workflow.Parameters {
	case "ENABLED", "DISABLED":
		dbKey.State = l.Workflow.Parameters
	default:
		return errs.Wrapf(ErrWorkflowExecution,
			"invalid key state "+l.Workflow.Parameters)
	}

	enabled := dbKey.State == "ENABLED"

	_, err = l.KeyActions.UpdateKey(ctx, keyID, cmkapi.KeyPatch{Enabled: &enabled})
	if err != nil {
		return errs.Wrap(ErrWorkflowExecution, err)
	}

	return nil
}

func (l *Lifecycle) deleteKey(ctx context.Context) error {
	err := l.KeyActions.Delete(ctx, l.Workflow.ArtifactID)
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
