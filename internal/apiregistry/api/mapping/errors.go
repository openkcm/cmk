package mapping

import (
	"errors"

	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
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

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) error {
	return apierrors.NewValidationError(field, message)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return apierrors.IsValidationError(err)
}
