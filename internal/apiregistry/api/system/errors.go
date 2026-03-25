package system

import (
	"errors"
	"fmt"
)

var (
	ErrSystemNotFound            = errors.New("system not found")
	ErrSystemAlreadyExists       = errors.New("system already exists")
	ErrInvalidRegion             = errors.New("invalid region")
	ErrInvalidExternalID         = errors.New("invalid external ID")
	ErrInvalidTenantID           = errors.New("invalid tenant ID")
	ErrL1KeyClaimAlreadyActive   = errors.New("L1 key claim is already active")
	ErrL1KeyClaimAlreadyInactive = errors.New("L1 key claim is already inactive")
	ErrSystemNotLinkedToTenant   = errors.New("system is not linked to the tenant")
	ErrInvalidSystemType         = errors.New("invalid system type")
	ErrInvalidLimit              = errors.New("invalid limit: must be positive or zero")
	ErrOperationFailed           = errors.New("operation failed")
)

type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
