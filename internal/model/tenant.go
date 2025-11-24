package model

import (
	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
)

var TenantTableName = "public.tenants"

type Tenant struct {
	multitenancy.TenantModel

	ID        string       `gorm:"type:varchar(255);not null;unique"`
	Region    string       `gorm:"type:varchar(50);not null"`
	Status    TenantStatus `gorm:"type:varchar(50);not null"`
	OwnerType string       `gorm:"type:varchar(50);not null;default:''"`
	OwnerID   string       `gorm:"type:varchar(255);not null;default:''"`
	IssuerURL string       `gorm:"type:varchar(255);not null;default:''"`
	Role      TenantRole   `gorm:"type:varchar(50);not null;default:''"`
}

// Validate validates given tenant data.
func (t Tenant) Validate() error {
	return ValidateAll(t.Status, t.Role)
}

func (t Tenant) TableName() string   { return TenantTableName }
func (t Tenant) IsSharedModel() bool { return true }
