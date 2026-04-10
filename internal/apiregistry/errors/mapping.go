package errors

import (
	"errors"
)

var (
	ErrMappingNotFound          = errors.New("mapping not found")
	ErrMappingAlreadyExists     = errors.New("mapping already exists")
	ErrMappingInvalidExternalID = errors.New("invalid external ID")
	ErrInvalidType              = errors.New("invalid type")
	ErrMappingInvalidTenantID   = errors.New("invalid tenant ID")
	ErrSystemNotMapped          = errors.New("system is not mapped to tenant")
	ErrMappingOperationFailed   = errors.New("operation failed")
)
