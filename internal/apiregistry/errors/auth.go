package errors

import (
	"errors"
)

var (
	ErrAuthNotFound          = errors.New("auth not found")
	ErrAuthAlreadyExists     = errors.New("auth already exists")
	ErrAuthInvalidExternalID = errors.New("invalid external ID")
	ErrAuthInvalidTenantID   = errors.New("invalid tenant ID")
	ErrAuthInvalidType       = errors.New("invalid auth type")
	ErrAuthInvalidProperties = errors.New("invalid auth properties")
	ErrAuthInvalidLimit      = errors.New("invalid limit: must be between 1 and 1000")
	ErrAuthOperationFailed   = errors.New("operation failed")
)
