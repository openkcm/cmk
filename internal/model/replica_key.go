package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

type ReplicaKey struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	KeyID    uuid.UUID `gorm:"type:uuid;not null"`
	NativeID string    `gorm:"type:varchar(255);not null"`
}

func (ReplicaKey) TableName() string {
	return "replica_keys"
}

func (ReplicaKey) IsSharedModel() bool {
	return false
}

func (ReplicaKey) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeKey
}

func (m ReplicaKey) CheckAuthz(ctx context.Context,
	handler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, handler, m.TableResourceType(), action)
}

type JoinKeyAndReplicas struct {
	Key

	Replica *ReplicaKey `gorm:"embedded;embeddedPrefix:replica_"`
}
