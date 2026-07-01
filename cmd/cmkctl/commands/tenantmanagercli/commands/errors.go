package commands

import "errors"

var (
	ErrTenantIDRequired = errors.New("tenant id is required")
	ErrDeleteTenant     = errors.New("failed to delete tenant")
	ErrTenantNotFound   = errors.New("tenant not found")
)
