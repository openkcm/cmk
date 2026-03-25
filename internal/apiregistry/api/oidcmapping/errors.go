package oidcmapping

import (
	"errors"
	"fmt"
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
