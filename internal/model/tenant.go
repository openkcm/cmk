package model

import (
	"context"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/authz"
)

type Tenant struct {
	multitenancy.TenantModel

	ID        string       `gorm:"type:varchar(255);not null;unique"`
	Name      string       `gorm:"type:varchar(255)"`
	Status    TenantStatus `gorm:"type:varchar(50);not null"`
	OwnerType string       `gorm:"type:varchar(50);not null;default:''"`
	OwnerID   string       `gorm:"type:varchar(255);not null;default:''"`
	IssuerURL string       `gorm:"type:varchar(255);not null;default:''"`
	Role      TenantRole   `gorm:"type:varchar(50);not null;default:''"`
}

// Validate validates given tenant data.
func (m Tenant) Validate() error {
	return ValidateAll(m.Status, m.Role)
}

// TableResourceType return the authz resource type
func (m Tenant) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeTenant
}

func (m Tenant) TableName() string {
	return string(m.TableResourceType())
}

func (m Tenant) IsSharedModel() bool { return true }

func (m Tenant) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
