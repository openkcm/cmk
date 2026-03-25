package oidcmapping

import (
	"errors"

	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

var (
	ErrOIDCMappingNotFound       = errors.New("OIDC mapping not found")
	ErrOIDCMappingAlreadyExists  = errors.New("OIDC mapping already exists")
	ErrInvalidTenantID           = errors.New("invalid tenant ID")
	ErrInvalidIssuer             = errors.New("invalid issuer")
	ErrInvalidJwksURI            = errors.New("invalid JWKS URI")
	ErrInvalidAudiences          = errors.New("invalid audiences")
	ErrInvalidClientID           = errors.New("invalid client ID")
	ErrOIDCMappingAlreadyBlocked = errors.New("OIDC mapping is already blocked")
	ErrOIDCMappingNotBlocked     = errors.New("OIDC mapping is not blocked")
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
