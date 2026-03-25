package errors

import (
	"errors"
	"fmt"
)

// ValidationError represents a validation error on a specific field.
type ValidationError struct {
	Field   string
	Message string
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
