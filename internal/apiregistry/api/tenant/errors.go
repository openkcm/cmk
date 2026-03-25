package tenant

import (
	"errors"

	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

var (
	ErrTenantNotFound          = errors.New("tenant not found")
	ErrTenantAlreadyExists     = errors.New("tenant already exists")
	ErrInvalidTenantID         = errors.New("invalid tenant ID")
	ErrInvalidRegion           = errors.New("invalid region")
	ErrInvalidOwnerID          = errors.New("invalid owner ID")
	ErrInvalidOwnerType        = errors.New("invalid owner type")
	ErrInvalidTenantName       = errors.New("invalid tenant name")
	ErrInvalidLimit            = errors.New("invalid limit: must be between 1 and 1000")
	ErrTenantAlreadyBlocked    = errors.New("tenant is already blocked")
	ErrTenantNotBlocked        = errors.New("tenant is not blocked")
	ErrTenantAlreadyTerminated = errors.New("tenant is already terminated")
	ErrInvalidTenantStatus     = errors.New("invalid tenant status for this operation")
	ErrInvalidTenantRole       = errors.New("invalid tenant role")
	ErrInvalidLabels           = errors.New("invalid labels")
	ErrInvalidLabelKeys        = errors.New("invalid label keys")
	ErrInvalidUserGroups       = errors.New("invalid user groups")
	ErrOperationFailed         = errors.New("operation failed")
)

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) error {
	return apierrors.NewValidationError(field, message)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return apierrors.IsValidationError(err)
}
