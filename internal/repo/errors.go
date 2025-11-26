package repo

import "errors"

var (
	ErrMigratingTenantModels = errors.New("migrating tenant models for existing tenant")
	ErrSchemaNameLength      = errors.New("schema name length must be between 3 and 63 characters")
	ErrCreatingTenant        = errors.New("creating tenant failed")
	ErrOnboardingTenant      = errors.New("onboarding tenant failed")
	ErrOnboardingInProgress  = errors.New("another onboarding is already in progress")
	ErrValidatingTenant      = errors.New("validating tenant failed")
)
