package workflow

// Persisted workflow enums live in internal/model. This file holds the
// FSM-only vocabulary.

// Transition is a state-machine event name.
type Transition string

func (t Transition) String() string { return string(t) }

const (
	TransitionCreate  Transition = "CREATE"
	TransitionRevoke  Transition = "REVOKE"
	TransitionReject  Transition = "REJECT"
	TransitionExpire  Transition = "EXPIRE"
	TransitionApprove Transition = "APPROVE"
	TransitionConfirm Transition = "CONFIRM"
	TransitionExecute Transition = "EXECUTE"
	TransitionFail    Transition = "FAIL"
)

var Transitions = []Transition{
	TransitionCreate, TransitionRevoke, TransitionReject,
	TransitionExpire, TransitionApprove, TransitionConfirm, TransitionExecute, TransitionFail,
}
