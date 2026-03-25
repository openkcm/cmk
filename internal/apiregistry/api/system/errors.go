package system

import (
	"errors"

	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
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

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) error {
	return apierrors.NewValidationError(field, message)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return apierrors.IsValidationError(err)
}
