package workflow

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrInvalidState is returned when a workflow state value is not recognized.
	ErrInvalidState = errors.New("invalid workflow state")
	// ErrInvalidArtifactType is returned when a workflow artifact type value is not recognized.
	ErrInvalidArtifactType = errors.New("invalid workflow artifact type")
	// ErrInvalidActionType is returned when a workflow action type value is not recognized.
	ErrInvalidActionType = errors.New("invalid workflow action type")
	// ErrUnexpectedScanType is returned when Scan receives a value that is not string or []byte.
	ErrUnexpectedScanType = errors.New("unexpected scan type")
)

// State represents the state of a workflow in the state-machine.
//
//nolint:recvcheck
type State string

func (s State) String() string {
	return string(s)
}

// Valid returns true if the state is a known workflow state.
func (s State) Valid() bool {
	return slices.Contains(States, s)
}

// Value implements the driver.Valuer interface.
func (s State) Value() (driver.Value, error) {
	if s == "" {
		return "", nil
	}
	if !s.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidState, s)
	}
	return string(s), nil
}

// Scan implements the sql.Scanner interface.
func (s *State) Scan(value any) error {
	if value == nil {
		*s = ""
		return nil
	}
	var sv string
	switch v := value.(type) {
	case string:
		sv = v
	case []byte:
		sv = string(v)
	default:
		return fmt.Errorf("%w: expected string or []byte, got %T", ErrUnexpectedScanType, value)
	}
	*s = State(sv)
	if !s.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidState, sv)
	}
	return nil
}

// Transition represents the transition of a workflow in the state-machine.
type Transition string

func (t Transition) String() string {
	return string(t)
}

// ArtifactType represents the type of the artifact that the workflow is acting on.
//
//nolint:recvcheck
type ArtifactType string

func (t ArtifactType) String() string {
	return string(t)
}

// Valid returns true if the artifact type is known.
func (t ArtifactType) Valid() bool {
	return slices.Contains(ArtifactTypes, t)
}

// Value implements the driver.Valuer interface.
func (t ArtifactType) Value() (driver.Value, error) {
	if t == "" {
		return "", nil
	}
	if !t.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidArtifactType, t)
	}
	return string(t), nil
}

// Scan implements the sql.Scanner interface.
func (t *ArtifactType) Scan(value any) error {
	if value == nil {
		*t = ""
		return nil
	}
	var sv string
	switch v := value.(type) {
	case string:
		sv = v
	case []byte:
		sv = string(v)
	default:
		return fmt.Errorf("%w: expected string or []byte, got %T", ErrUnexpectedScanType, value)
	}
	*t = ArtifactType(sv)
	if !t.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidArtifactType, sv)
	}
	return nil
}

// ParametersResourceType represents the type of the resource that is referenced in the workflow parameters.
type ParametersResourceType string

func (t ParametersResourceType) String() string {
	return string(t)
}

// ActionType represents the type of the action that the workflow is performing.
//
//nolint:recvcheck
type ActionType string

func (t ActionType) String() string {
	return string(t)
}

// Valid returns true if the action type is known.
func (t ActionType) Valid() bool {
	return slices.Contains(ActionTypes, t)
}

// Value implements the driver.Valuer interface.
func (t ActionType) Value() (driver.Value, error) {
	if t == "" {
		return "", nil
	}
	if !t.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidActionType, t)
	}
	return string(t), nil
}

// Scan implements the sql.Scanner interface.
func (t *ActionType) Scan(value any) error {
	if value == nil {
		*t = ""
		return nil
	}
	var sv string
	switch v := value.(type) {
	case string:
		sv = v
	case []byte:
		sv = string(v)
	default:
		return fmt.Errorf("%w: expected string or []byte, got %T", ErrUnexpectedScanType, value)
	}
	*t = ActionType(sv)
	if !t.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidActionType, sv)
	}
	return nil
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

	ParametersResourceTypeKey              ParametersResourceType = "KEY"
	ParametersResourceTypeKeyConfiguration ParametersResourceType = "KEY_CONFIGURATION"

	ActionTypeUpdateState   ActionType = "UPDATE_STATE"
	ActionTypeUpdatePrimary ActionType = "UPDATE_PRIMARY"
	ActionTypeLink          ActionType = "LINK"
	ActionTypeUnlink        ActionType = "UNLINK"
	ActionTypeSwitch        ActionType = "SWITCH"
	ActionTypeDelete        ActionType = "DELETE"
)

var States = []State{
	StateInitial, StateRevoked, StateRejected, StateExpired, StateWaitApproval,
	StateWaitConfirmation, StateExecuting, StateSuccessful, StateFailed,
}

var Transitions = []Transition{
	TransitionCreate, TransitionRevoke, TransitionReject,
	TransitionExpire, TransitionApprove, TransitionConfirm, TransitionExecute, TransitionFail,
}

var ArtifactTypes = []ArtifactType{ArtifactTypeKey, ArtifactTypeKeyConfiguration, ArtifactTypeSystem}

var ActionTypes = []ActionType{
	ActionTypeUpdateState, ActionTypeUpdatePrimary,
	ActionTypeLink, ActionTypeUnlink, ActionTypeSwitch, ActionTypeDelete,
}

var NonTerminalStates = []string{
	StateInitial.String(),
	StateWaitApproval.String(),
	StateWaitConfirmation.String(),
	StateExecuting.String(),
}

var TerminalStates = []string{
	StateRevoked.String(),
	StateRejected.String(),
	StateExpired.String(),
	StateSuccessful.String(),
	StateFailed.String(),
}
