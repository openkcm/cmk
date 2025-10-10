package model

import (
	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
)

type Tenant struct {
	multitenancy.TenantModel

	ID        string       `gorm:"type:varchar(255);not null;unique"`
	Region    string       `gorm:"type:varchar(50);not null"`
	Status    TenantStatus `gorm:"type:varchar(50);not null"`
	OwnerType string       `gorm:"type:varchar(50);not null;default:''"`
	OwnerID   string       `gorm:"type:varchar(255);not null;default:''"`
}

func (t Tenant) TableName() string   { return "public.tenants" }
func (t Tenant) IsSharedModel() bool { return true }
