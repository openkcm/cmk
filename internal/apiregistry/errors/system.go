package errors

import (
	"errors"
)

var (
	ErrSystemNotFound            = errors.New("system not found")
	ErrSystemAlreadyExists       = errors.New("system already exists")
	ErrSystemInvalidRegion       = errors.New("invalid region")
	ErrInvalidExternalID         = errors.New("invalid external ID")
	ErrSystemInvalidTenantID     = errors.New("invalid tenant ID")
	ErrL1KeyClaimAlreadyActive   = errors.New("L1 key claim is already active")
	ErrL1KeyClaimAlreadyInactive = errors.New("L1 key claim is already inactive")
	ErrSystemNotLinkedToTenant   = errors.New("system is not linked to the tenant")
	ErrInvalidSystemType         = errors.New("invalid system type")
	ErrSystemInvalidLimit        = errors.New("invalid limit: must be positive or zero")
	ErrSystemOperationFailed     = errors.New("operation failed")
)
