package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// WorkflowApproverGroup represents the many-to-many relationship between workflows and approver groups
type WorkflowApproverGroup struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	WorkflowID uuid.UUID `gorm:"type:uuid;not null"`
	GroupID    uuid.UUID `gorm:"type:uuid;not null"`
}

func (w WorkflowApproverGroup) TableName() string {
	return "workflow_approver_groups"
}

func (w WorkflowApproverGroup) IsSharedModel() bool {
	return false
}

func (w WorkflowApproverGroup) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeWorkflow
}

func (w WorkflowApproverGroup) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, w.TableResourceType(), action)
}
