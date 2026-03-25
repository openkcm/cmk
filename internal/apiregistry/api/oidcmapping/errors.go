package oidcmapping

import "fmt"

var (
	ErrOIDCMappingNotFound = fmt.Errorf("OIDC mapping not found")
	ErrOIDCMappingAlreadyExists = fmt.Errorf("OIDC mapping already exists")
	ErrInvalidTenantID = fmt.Errorf("invalid tenant ID")
	ErrInvalidIssuer = fmt.Errorf("invalid issuer")
	ErrInvalidJwksURI = fmt.Errorf("invalid JWKS URI")
	ErrInvalidAudiences = fmt.Errorf("invalid audiences")
	ErrInvalidClientID = fmt.Errorf("invalid client ID")
	ErrOIDCMappingAlreadyBlocked = fmt.Errorf("OIDC mapping is already blocked")
	ErrOIDCMappingNotBlocked = fmt.Errorf("OIDC mapping is not blocked")
	ErrOperationFailed = fmt.Errorf("operation failed")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}
