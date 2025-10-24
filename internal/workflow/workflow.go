package workflow

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/looplab/fsm"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

var SystemUserID = uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

type Lifecycle struct {
	Workflow                *model.Workflow
	StateMachine            *fsm.FSM
	ActorID                 uuid.UUID
	Repository              repo.Repo
	KeyActions              KeyActions
	KeyConfigurationActions KeyConfigurationActions
	SystemActions           SystemActions
	MinimumApproverCount    int
}

// convertEvent converts Transition and State types to string
// and creates an EventDesc object for the state machine.
func convertEvent(
	transition Transition,
	sourceStates []State,
	destinationState State,
) fsm.EventDesc {
	src := make([]string, len(sourceStates))
	for i, state := range sourceStates {
		src[i] = state.String()
	}

	return fsm.EventDesc{
		Name: transition.String(),
		Src:  src,
		Dst:  destinationState.String(),
	}
}

// NewLifecycle creates a new Lifecycle object for the given workflow
// with a state machine that defines the possible transitions.
//
//nolint:funlen
func NewLifecycle(workflow *model.Workflow,
	keyActions KeyActions,
	keyConfigurationActions KeyConfigurationActions,
	systemActions SystemActions,
	repo repo.Repo,
	actorID uuid.UUID,
	minimumApproverCount int,
) *Lifecycle {
	stateMachine := fsm.NewFSM(
		workflow.State,
		fsm.Events{
			convertEvent(
				TransitionCreate,
				[]State{StateInitial},
				StateWaitApproval,
			),
			convertEvent(
				TransitionApprove,
				[]State{StateWaitApproval},
				StateWaitConfirmation,
			),
			convertEvent(
				TransitionReject,
				[]State{StateWaitApproval},
				StateRejected,
			),
			convertEvent(
				TransitionRevoke,
				[]State{StateWaitApproval, StateWaitConfirmation},
				StateRevoked,
			),
			convertEvent(
				TransitionConfirm,
				[]State{StateWaitConfirmation},
				StateExecuting,
			),
			convertEvent(
				TransitionExpire,
				[]State{StateWaitApproval, StateWaitConfirmation, StateExecuting},
				StateExpired,
			),
			convertEvent(
				TransitionFail,
				[]State{StateExecuting},
				StateFailed,
			),
			convertEvent(
				TransitionExecute,
				[]State{StateExecuting},
				StateSuccessful,
			),
		},
		fsm.Callbacks{},
	)

	// If the minimum approver count is not set, default to 2
	if minimumApproverCount == 0 {
		minimumApproverCount = 2
	}

	return &Lifecycle{
		Workflow:                workflow,
		StateMachine:            stateMachine,
		KeyActions:              keyActions,
		KeyConfigurationActions: keyConfigurationActions,
		SystemActions:           systemActions,
		ActorID:                 actorID,
		Repository:              repo,
		MinimumApproverCount:    minimumApproverCount,
	}
}

// CanTransition checks if the workflow can transition to the given state
func (l *Lifecycle) CanTransition(transition Transition) bool {
	return l.StateMachine.Can(transition.String())
}

// ApplyTransition wraps the execution of a transition in the state machine
// triggered by user input
func (l *Lifecycle) ApplyTransition(ctx context.Context, transition Transition) error {
	// Validate the actor of the event
	err := l.ValidateActor(ctx, transition)
	if err != nil {
		return err
	}

	// Perform pre-checks on the transition
	skip, err := l.transitionPrecheck(ctx, transition)
	if err != nil {
		return err
	} else if skip {
		return nil
	}

	// Execute the transition in the state machine
	transitionErr := l.StateMachine.Event(ctx, transition.String())
	if transitionErr != nil {
		return errs.Wrap(NewTransitionError(transition), transitionErr)
	}

	// If the workflow is now in the EXECUTING state, execute the action
	// and transition to next state based on the result
	if l.StateMachine.Current() == StateExecuting.String() {
		// Transitioning to either SUCCESSFUL or FAILED does not require any validation
		// because EXECUTING -> SUCCESSFUL and EXECUTING -> FAILED are
		// guaranteed to be valid transitions.
		// Therefore, if an error is returned, it is unexpected and must be logged.
		err = l.transitionExecute(ctx)
		if err != nil {
			log.Error(ctx, "unexpected error when applying Executing transition", err)
			return err
		}
	}

	// Update the workflow state in the database
	l.Workflow.State = l.StateMachine.Current()

	_, err = l.Repository.Patch(ctx, l.Workflow, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrUpdateWorkflowState, err)
	}

	return nil
}

// Expire triggers to EXPIRED state
func (l *Lifecycle) Expire(ctx context.Context) error {
	err := l.StateMachine.Event(ctx, TransitionExpire.String())
	if err != nil {
		return errs.Wrap(NewTransitionError(TransitionExpire), err)
	}

	l.Workflow.State = l.StateMachine.Current()

	_, err = l.Repository.Patch(ctx, l.Workflow, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrUpdateWorkflowState, err)
	}

	return nil
}

// ValidateActor validates the actor of the event
//
//nolint:cyclop
func (l *Lifecycle) ValidateActor(ctx context.Context, transition Transition) error {
	var (
		valid bool
		err   error
	)

	switch transition {
	case TransitionCreate, TransitionRevoke, TransitionConfirm:
		valid, err = l.validateUserIsInitiator(ctx)
		if err != nil {
			err = errs.Wrapf(err, "failed to validate initiator")
		} else if !valid {
			err = NewInvalidEventActorError(l.ActorID, "initiator")
		}
	case TransitionApprove, TransitionReject:
		valid, err = l.validateUserIsApprover(ctx)
		if err != nil {
			err = errs.Wrapf(err, "failed to validate approver")
		} else if !valid {
			err = NewInvalidEventActorError(l.ActorID, "approver")
		}
	case TransitionExecute, TransitionFail, TransitionExpire:
		valid, err = l.validateUserIsSystem(ctx)
		if err != nil {
			err = errs.Wrapf(err, "failed to validate automated transition")
		} else if !valid {
			err = ErrAutomatedTransition
		}
	default:
		err = ErrInvalidWorkflowState
	}

	return err
}

// validateUserIsSystem validates that the user is the SYSTEM user
//
//nolint:unparam
func (l *Lifecycle) validateUserIsSystem(_ context.Context) (bool, error) {
	return l.ActorID == SystemUserID, nil
}

// validateUserIsInitiator validates that the user is the initiator of the workflow
//
//nolint:unparam
func (l *Lifecycle) validateUserIsInitiator(_ context.Context) (bool, error) {
	return l.ActorID == l.Workflow.InitiatorID, nil
}

// validateUserIsApprover validates that the user is an approver of the workflow
func (l *Lifecycle) validateUserIsApprover(ctx context.Context) (bool, error) {
	ck := repo.NewCompositeKey().Where(
		fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), l.Workflow.ID).Where(
		fmt.Sprintf("%s_%s", repo.UserField, repo.IDField), l.ActorID)

	count, err := l.Repository.List(
		ctx,
		model.WorkflowApprover{},
		&[]model.WorkflowApprover{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return false, errs.Wrap(ErrCheckApprovers, err)
	}

	return count > 0, nil
}

// transitionPrecheck performs pre-checks on the transition before
// passing to the state machine. Returns true if the transition should be skipped.
//
//nolint:cyclop
func (l *Lifecycle) transitionPrecheck(ctx context.Context, transition Transition) (bool, error) {
	switch transition {
	case TransitionCreate:
		// Check if the workflow has enough approvers before transitioning from INITIAL to WAIT_APPROVAL
		approversCount, err := l.getNumberOfApprovers(ctx)

		switch {
		case err != nil:
			return false, err
		case approversCount < l.MinimumApproverCount:
			err = NewInsufficientApproverCountError(approversCount, l.MinimumApproverCount)
			return false, err
		default:
			return false, nil
		}
	case TransitionApprove:
		// Check if all approvers have made decisions before transitioning from WAIT_APPROVAL to WAIT_CONFIRMATION
		canTransition, err := l.canTransitionApprove(ctx)
		if err != nil {
			return true, err
		} else if !canTransition {
			return true, nil
		}
	case TransitionExecute, TransitionFail, TransitionExpire:
		// Forbid automated transitions from being triggered by user input
		err := NewTransitionError(transition)
		return true, errs.Wrapf(err, "automated transition cannot be triggered by user input")
	case TransitionConfirm, TransitionRevoke, TransitionReject:
		// No pre-checks required for other transitions
		fallthrough
	default:
		return false, nil
	}

	return false, nil
}

// transitionExecute transitions the workflow from EXECUTING to
// either SUCCESSFUL or FAILED depending on the result
func (l *Lifecycle) transitionExecute(ctx context.Context) error {
	executionErr := l.executeWorkflowAction(ctx)

	var (
		transition    Transition
		terminalError error
	)

	if executionErr != nil {
		transition = TransitionFail
		l.Workflow.FailureReason = executionErr.Error()
	} else {
		transition = TransitionExecute
	}

	terminalError = l.StateMachine.Event(ctx, transition.String())
	if terminalError != nil {
		return errs.Wrap(NewTransitionError(transition), terminalError)
	}

	return nil
}

// canTransitionApprove checks if all approvers have approved the workflow
func (l *Lifecycle) canTransitionApprove(ctx context.Context) (bool, error) {
	if l.StateMachine.Cannot(TransitionApprove.String()) {
		fsmErr := fsm.InvalidEventError{Event: TransitionApprove.String(), State: l.Workflow.State}
		return false, errs.Wrap(NewTransitionError(TransitionApprove), fsmErr)
	}

	_, err := l.Repository.First(ctx, &model.Workflow{ID: l.Workflow.ID}, *repo.NewQuery())
	if err != nil {
		return false, errs.Wrap(ErrCheckApproverDecision, err)
	}

	approvers := []*model.WorkflowApprover{}
	ck := repo.NewCompositeKey().Where(
		fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), l.Workflow.ID).Where(
		repo.ApprovedField, repo.FalseNull)

	count, err := l.Repository.List(
		ctx,
		model.WorkflowApprover{},
		&approvers,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return false, errs.Wrap(ErrCheckApproverDecision, err)
	}

	return count == 0, nil
}

// getNumberOfApprovers gets the number of approvers for the workflow
func (l *Lifecycle) getNumberOfApprovers(ctx context.Context) (int, error) {
	workflow := &model.Workflow{ID: l.Workflow.ID}

	_, err := l.Repository.First(ctx, workflow, *repo.NewQuery())
	if err != nil {
		return -1, errs.Wrap(ErrListApprovers, err)
	}

	ck := repo.NewCompositeKey().Where(
		fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), l.Workflow.ID)
	approvers := []*model.WorkflowApprover{}

	count, err := l.Repository.List(
		ctx,
		model.WorkflowApprover{},
		&approvers,
		*repo.NewQuery().
			SetLimit(constants.DefaultTop).
			Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return -1, errs.Wrap(ErrListApprovers, err)
	}

	return count, nil
}

type workflowHandlerFunc func(context.Context) error

func (l *Lifecycle) executeWorkflowAction(ctx context.Context) error {
	handlers := map[string]map[string]workflowHandlerFunc{
		ArtifactTypeKey.String(): {
			ActionTypeUpdateState.String():      l.updateKeyState,
			ActionTypeDelete.String():           l.deleteKey,
			ActionTypeUpdatePrimaryKey.String(): l.updatePrimaryKey,
		},
		ArtifactTypeKeyConfiguration.String(): {
			ActionTypeDelete.String(): l.deleteKeyConfiguration,
		},
		ArtifactTypeSystem.String(): {
			ActionTypeLink.String():   l.systemLinkOrSwitch,
			ActionTypeUnlink.String(): l.systemUnlink,
			ActionTypeSwitch.String(): l.systemLinkOrSwitch,
		},
	}

	artifactHandlers, ok := handlers[l.Workflow.ArtifactType]
	if !ok {
		return errs.Wrapf(
			ErrWorkflowExecution,
			"unknown artifact type "+l.Workflow.ArtifactType,
		)
	}

	handler, ok := artifactHandlers[l.Workflow.ActionType]
	if !ok {
		return errs.Wrapf(
			ErrWorkflowExecution,
			"unknown action type "+l.Workflow.ActionType,
		)
	}

	return handler(ctx)
}
