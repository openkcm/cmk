package cmd

import "errors"

var (
	ErrTenantIDRequired     = errors.New("tenant id is required")
	ErrTenantStatusRequired = errors.New("tenant status is required")
	ErrTenantRegionRequired = errors.New("tenant status is required")
	ErrDeleteTenant         = errors.New("failed to delete tenant")
	ErrTenantNotFound       = errors.New("tenant not found")
)
