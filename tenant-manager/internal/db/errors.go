package db

import "errors"

var (
	ErrValidatingSchema      = errors.New("validating schema name")
	ErrMigratingTenantModels = errors.New("migrating tenant models for existing tenant")
	ErrCreatingGroups        = errors.New("creating user groups for existing tenant")
	ErrSchemaNameLength      = errors.New("schema name length must be between 3 and 63 characters")
	ErrOnboardingInProgress  = errors.New("another onboarding is already in progress")
	ErrEmptyTenantID         = errors.New("tenantID cannot be empty")
	ErrInvalidGroupType      = errors.New("invalid group type")
	ErrCreatingTenant        = errors.New("creating tenant failed")
)
