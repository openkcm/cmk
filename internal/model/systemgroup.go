package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

const VirtualSystemGroup = "root"

type SystemGroup struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name            string    `gorm:"type:varchar(255);not null;unique"`
	Description     string    `gorm:"type:varchar(255)"`
	SuppressWarning bool      `gorm:"type:bool"`

	Systems      []*System `gorm:"-"`
	WarnCoverage bool      `gorm:"-"`
}

func (m SystemGroup) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeSystemGroup
}

func (m SystemGroup) TableName() string {
	return string(m.TableResourceType())
}

func (m SystemGroup) IsSharedModel() bool { return false }

func (m SystemGroup) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

type JoinSystemGroupsAndSystem struct {
	JoinSystemAndProperties

	SystemGroup SystemGroup `gorm:"embedded;embeddedPrefix:group_"`
}

func (m JoinSystemGroupsAndSystem) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeSystem
}

func (m JoinSystemGroupsAndSystem) TableName() string {
	return string(m.TableResourceType())
}

func (m JoinSystemGroupsAndSystem) IsSharedModel() bool { return false }

func (m JoinSystemGroupsAndSystem) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

type SystemGroupCoverage struct {
	SystemGroupID *uuid.UUID `gorm:"column:system_group_id"`
	WarnCoverage  bool       `gorm:"column:warn_coverage"`
}

func (SystemGroupCoverage) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeSystem
}

func (m SystemGroupCoverage) TableName() string { return string(m.TableResourceType()) }

func (SystemGroupCoverage) IsSharedModel() bool { return false }

func (m SystemGroupCoverage) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
