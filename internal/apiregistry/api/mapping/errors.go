package mapping

import (
	"errors"
	"fmt"
)

var (
	ErrMappingNotFound      = errors.New("mapping not found")
	ErrMappingAlreadyExists = errors.New("mapping already exists")
	ErrInvalidExternalID    = errors.New("invalid external ID")
	ErrInvalidType          = errors.New("invalid type")
	ErrInvalidTenantID      = errors.New("invalid tenant ID")
	ErrSystemNotMapped      = errors.New("system is not mapped to tenant")
	ErrOperationFailed      = errors.New("operation failed")
)

type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func IsValidationError(err error) bool {
	validationError := &ValidationError{}
	ok := errors.As(err, &validationError)
	return ok
}
