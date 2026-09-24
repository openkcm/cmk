package model

import (
	"context"
	"fmt"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/multitenancy"
)

var ErrInvalidSystemLimitOverride = fmt.Errorf("%w: system limit override must be >= 1", ErrValidation)

type Tenant struct {
	multitenancy.TenantModel

	ID                  string       `gorm:"type:varchar(255);not null;unique"`
	Name                string       `gorm:"type:varchar(255)"`
	Status              TenantStatus `gorm:"type:varchar(50);not null"`
	OwnerType           string       `gorm:"type:varchar(50);not null;default:''"`
	OwnerID             string       `gorm:"type:varchar(255);not null;default:''"`
	IssuerURL           string       `gorm:"type:varchar(255);not null;default:''"`
	Role                TenantRole   `gorm:"type:varchar(50);not null;default:''"`
	SystemLimitOverride *int         `gorm:"type:integer"`
}

// Validate validates given tenant data.
func (m Tenant) Validate() error {
	if err := ValidateAll(m.Status, m.Role); err != nil {
		return err
	}

	if m.SystemLimitOverride != nil && *m.SystemLimitOverride < config.MinTenantLimit {
		return ErrInvalidSystemLimitOverride
	}

	return nil
}

// TableResourceType return the authz resource type
func (m Tenant) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeTenant
}

func (m Tenant) TableName() string {
	return string(m.TableResourceType())
}

func (m Tenant) IsSharedModel() bool { return true }

func (m Tenant) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	// Read tenant actions can run without authorization checks
	switch action {
	case authz.RepoActionList, authz.RepoActionFirst, authz.RepoActionCount:
		return true, nil
	default:
		return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
	}
}
