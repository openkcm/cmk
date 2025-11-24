package errs

import (
	"errors"
	"fmt"

	keystoreErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"
)

type GRPCErrorCode string

// GRPCError represents a structured error for gRPC responses.
// The error can be matched using errors.Is and errors.As.
type GRPCError struct {
	Code        GRPCErrorCode
	BaseMessage string
	Reason      string
	Metadata    map[string]string
}

func (e GRPCError) Error() string {
	return fmt.Sprintf("%v: %v", e.BaseMessage, e.Reason)
}

func (e GRPCError) Is(target error) bool {
	if target == nil {
		return false
	}

	return errors.As(target, &GRPCError{})
}

func (e GRPCError) As(target any) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*GRPCError)

	return ok
}

func (e GRPCError) FromStatusError(err error) GRPCError {
	reason, metadata := keystoreErrs.GetDetails(err)

	e.Reason = reason
	e.Metadata = metadata

	return e
}

type JoinedError interface {
	Unwrap() []error
}

// GetGRPCErrorContext extracts the context from a GRPCError if it exists
// in a joined error chain.
func GetGRPCErrorContext(err error) map[string]any {
	joinedErr, ok := err.(JoinedError)
	if !ok || len(joinedErr.Unwrap()) < 2 {
		return nil
	}

	var e GRPCError

	ok = errors.As(joinedErr.Unwrap()[1], &e)
	if !ok {
		return nil
	}

	errContext := map[string]any{
		"reason": e.Reason,
	}

	for k, v := range e.Metadata {
		errContext[k] = v
	}

	return errContext
}
