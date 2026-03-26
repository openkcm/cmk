package errors

import (
	"errors"
)

var (
	ErrOIDCMappingNotFound       = errors.New("OIDC mapping not found")
	ErrOIDCMappingAlreadyExists  = errors.New("OIDC mapping already exists")
	ErrOIDCInvalidTenantID       = errors.New("invalid tenant ID")
	ErrInvalidIssuer             = errors.New("invalid issuer")
	ErrInvalidJwksURI            = errors.New("invalid JWKS URI")
	ErrInvalidAudiences          = errors.New("invalid audiences")
	ErrInvalidClientID           = errors.New("invalid client ID")
	ErrOIDCMappingAlreadyBlocked = errors.New("OIDC mapping is already blocked")
	ErrOIDCMappingNotBlocked     = errors.New("OIDC mapping is not blocked")
	ErrOIDCOperationFailed       = errors.New("operation failed")
)
