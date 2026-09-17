package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
)

// WorkflowKeyConfiguration represents the many-to-many relationship between workflows and key configurations
type WorkflowKeyConfiguration struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	WorkflowID         uuid.UUID `gorm:"type:uuid;not null"`
	KeyConfigurationID uuid.UUID `gorm:"type:uuid;not null"`
}

func (w WorkflowKeyConfiguration) TableName() string {
	return constants.WorkflowKeyConfigurationTable
}

func (w WorkflowKeyConfiguration) IsSharedModel() bool {
	return false
}

func (w WorkflowKeyConfiguration) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeWorkflowKeyConfiguration
}

func (w WorkflowKeyConfiguration) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, w.TableResourceType(), action)
}
