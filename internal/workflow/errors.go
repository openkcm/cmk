package workflow

import (
	"errors"
	"fmt"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrInvalidEventActor         = errors.New("invalid event actor")
	ErrInsufficientApproverCount = errors.New("insufficient approvers to transition to next state")
	ErrTransitionExecution       = errors.New("failed to execute transition")
	ErrWorkflowExecution         = errors.New("failed to execute workflow action")
	ErrUpdateWorkflowState       = errors.New("fialed to update workflow state")
	ErrCheckApprovers            = errors.New("failed to check approvers")
	ErrAutomatedTransition       = errors.New(
		"automated transition cannot be triggered by user input",
	)
	ErrInvalidWorkflowState    = errors.New("invalid workflow state")
	ErrInvalidWorkflowType     = errors.New("invalid workflow type")
	ErrCheckApproverDecision   = errors.New("failed to check approver decision")
	ErrListApprovers           = errors.New("failed to list approvers")
	ErrInvalidVotingTransition = errors.New("invalid voting transition")
)

// NewInvalidEventActorError creates an error when the user is not the expected actor of the event.
func NewInvalidEventActorError(userID string, expectedRole string) error {
	msg := fmt.Sprintf("user %s is not the %s of the workflow", userID, expectedRole)
	return errs.Wrapf(ErrInvalidEventActor, msg)
}

// NewInsufficientApproverCountError creates an error when there are not enough approvers
// to transition to the next state.
func NewInsufficientApproverCountError(currentCount, requiredCount int) error {
	msg := fmt.Sprintf("%d, required: %d", currentCount, requiredCount)
	return errs.Wrapf(ErrInsufficientApproverCount, msg)
}

// NewTransitionError creates an error when a transition fails.
func NewTransitionError(transition Transition) error {
	return fmt.Errorf("%w %s", ErrTransitionExecution, transition)
}
