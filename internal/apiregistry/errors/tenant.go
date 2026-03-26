package errors

import (
	"errors"
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
	ErrTenantOperationFailed   = errors.New("operation failed")
)
