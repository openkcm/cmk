package workflow

// State represents the state of a workflow in the state-machine.
type State string

func (s State) String() string {
	return string(s)
}

// Transition represents the transition of a workflow in the state-machine.
type Transition string

func (t Transition) String() string {
	return string(t)
}

// ArtifactType represents the type of the artifact that the workflow is acting on.
type ArtifactType string

func (t ArtifactType) String() string {
	return string(t)
}

// ActionType represents the type of the action that the workflow is performing.
type ActionType string

func (t ActionType) String() string {
	return string(t)
}

const (
	StateInitial          State = "INITIAL"
	StateRevoked          State = "REVOKED"
	StateRejected         State = "REJECTED"
	StateExpired          State = "EXPIRED"
	StateWaitApproval     State = "WAIT_APPROVAL"
	StateWaitConfirmation State = "WAIT_CONFIRMATION"
	StateExecuting        State = "EXECUTING"
	StateSuccessful       State = "SUCCESSFUL"
	StateFailed           State = "FAILED"

	TransitionCreate  Transition = "CREATE"
	TransitionRevoke  Transition = "REVOKE"
	TransitionReject  Transition = "REJECT"
	TransitionExpire  Transition = "EXPIRE"
	TransitionApprove Transition = "APPROVE"
	TransitionConfirm Transition = "CONFIRM"
	TransitionExecute Transition = "EXECUTE"
	TransitionFail    Transition = "FAIL"

	ArtifactTypeKey              ArtifactType = "KEY"
	ArtifactTypeKeyConfiguration ArtifactType = "KEY_CONFIGURATION"
	ArtifactTypeSystem           ArtifactType = "SYSTEM"

	ActionTypeUpdateState      ActionType = "UPDATE_STATE"
	ActionTypeUpdatePrimaryKey ActionType = "UPDATE_PRIMARY_KEY"
	ActionTypeLink             ActionType = "LINK"
	ActionTypeUnlink           ActionType = "UNLINK"
	ActionTypeSwitch           ActionType = "SWITCH"
	ActionTypeDelete           ActionType = "DELETE"
)

var (
	NonTerminalStates = []string{
		StateInitial.String(),
		StateWaitApproval.String(),
		StateWaitConfirmation.String(),
		StateExecuting.String(),
	}
)
