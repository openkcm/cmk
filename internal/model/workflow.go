package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	cmkContext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

const WorkflowID = "workflow_id"

// Workflow is an action on a data model (artifact) and can be read as <Artifact><ActionType>
// Artifact type is the type of item, identified by ArtifactID and ActionType the executed action
// Parameters will have different values depending on the ActionType.
// Check API Yaml for possible Parameters
//
// e.g. of a workflow
// System Link
//
//nolint:recvcheck
type Workflow struct {
	AutoTimeModel

	ID                     uuid.UUID          `gorm:"type:uuid;primaryKey"`
	State                  string             `gorm:"type:varchar(50);not null"`
	InitiatorID            string             `gorm:"type:varchar(255);not null"`
	initiatorName          string             `gorm:"-:all"`
	Approvers              []WorkflowApprover `gorm:"foreignKey:WorkflowID"`
	ApproverGroupIDs       json.RawMessage    `gorm:"type:jsonb"`
	ArtifactType           string             `gorm:"type:varchar(50);not null"`
	ArtifactID             uuid.UUID          `gorm:"type:uuid;not null"`
	ArtifactName           *string            `gorm:"type:varchar(255)"` // Currently a snapshot at time of creation
	ActionType             string             `gorm:"type:varchar(50);not null"`
	Parameters             string             `gorm:"type:text"`
	ParametersResourceName *string            `gorm:"type:varchar(255)"`
	ParametersResourceType *string            `gorm:"type:varchar(50)"`
	FailureReason          string             `gorm:"type:text"`
	ExpiryDate             *time.Time
}

// TableResourceType return the authz resource type
func (m Workflow) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeWorkflow
}

func (m Workflow) TableName() string {
	return string(m.TableResourceType())
}

func (Workflow) IsSharedModel() bool { return false }

func (m Workflow) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

func (m Workflow) BeforeDelete(tx *gorm.DB) error {
	// Delete all associated workflow approvers
	return tx.Where(WorkflowID+" = ?", m.ID).Delete(&WorkflowApprover{}).Error
}

func (m *Workflow) BeforeSave(tx *gorm.DB) error {
	if m.ExpiryDate == nil {
		m.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, constants.DefaultExpiryPeriodDays))
	}
	return nil
}

// Description generates a human-readable description of the workflow based on its action type
func (m Workflow) Description(
	ctx context.Context,
	idm identitymanagement.IdentityManagement,
) (string, error) {
	// Build workflow description based on artifact type first, then action type
	var description string
	var err error

	switch m.ArtifactType {
	case constants.WorkflowArtifactTypeSystem:
		description, err = m.buildSystemDescription(ctx, idm)
		if err != nil {
			return "", err
		}
	default:
		description, err = m.buildDefaultDescription(ctx, idm)
		if err != nil {
			return "", err
		}
	}

	description += "."

	return description, nil
}

// GetArtifactName returns the artifact name or a default value if nil
func (m Workflow) GetArtifactName() string {
	if m.ArtifactName != nil {
		return *m.ArtifactName
	}
	return ""
}

func (w *Workflow) GetInitiatorName(
	ctx context.Context,
	identityManager identitymanagement.IdentityManagement,
) (string, error) {
	if w.initiatorName != "" {
		return w.initiatorName, nil
	}

	authCtx, err := cmkContext.ExtractClientDataAuthContext(ctx)
	if err != nil {
		return "", err
	}
	user, err := identityManager.GetUser(ctx, &identitymanagement.GetUserRequest{
		UserID:      w.InitiatorID,
		AuthContext: identitymanagement.AuthContext{Data: authCtx},
	})
	if err != nil {
		return "", err
	}
	return user.User.Name, nil
}

// buildSystemDescription generates a description for SYSTEM artifact workflows
func (m Workflow) buildSystemDescription(
	ctx context.Context,
	idm identitymanagement.IdentityManagement,
) (string, error) {
	var description string
	artifactName := m.GetArtifactName()

	initiatorName, err := m.GetInitiatorName(ctx, idm)
	if err != nil {
		return "", err
	}

	description = fmt.Sprintf("%s requested approval to %s %s",
		initiatorName,
		m.ActionType,
		m.ArtifactType,
	)

	if artifactName != "" {
		description += fmt.Sprintf(": '%s'", artifactName)
	}

	if m.Parameters != "" {
		resourceType := m.getParametersResourceType()
		resourceName := m.getParametersResourceName()
		if resourceType != "" && resourceName != "" {
			description += fmt.Sprintf(" to %s: '%s'", resourceType, resourceName)
		}
	}

	return description, nil
}

// getParametersResourceType returns the parameters resource type or empty string if nil
func (m Workflow) getParametersResourceType() string {
	if m.ParametersResourceType != nil {
		return *m.ParametersResourceType
	}
	return ""
}

// getParametersResourceName returns the parameters resource name or empty string if nil
func (m Workflow) getParametersResourceName() string {
	if m.ParametersResourceName != nil {
		return *m.ParametersResourceName
	}
	return ""
}

// buildDefaultDescription generates a default description for workflows
func (m Workflow) buildDefaultDescription(
	ctx context.Context,
	idm identitymanagement.IdentityManagement,
) (string, error) {
	artifactName := m.GetArtifactName()

	initiatorName, err := m.GetInitiatorName(ctx, idm)
	if err != nil {
		return "", err
	}

	description := fmt.Sprintf("%s requested approval to %s %s",
		initiatorName,
		m.ActionType,
		m.ArtifactType,
	)

	if artifactName != "" {
		description += fmt.Sprintf(": '%s'", artifactName)
	}

	if m.Parameters != "" {
		description += " with parameters: " + m.Parameters
	}

	return description, nil
}

//nolint:recvcheck
type WorkflowApprover struct {
	WorkflowID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     string    `gorm:"type:varchar(255);primaryKey"`
	userName   string    `gorm:"-:all"`

	Workflow Workflow     `gorm:"foreignKey:WorkflowID"`
	Approved sql.NullBool `gorm:"default:null"`
}

// TableResourceType return the authz resource type
func (m WorkflowApprover) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeWorkflowApprover
}

func (m WorkflowApprover) TableName() string {
	return string(m.TableResourceType())
}

func (WorkflowApprover) IsSharedModel() bool { return false }

func (m WorkflowApprover) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

func (m *WorkflowApprover) GetUserName(
	ctx context.Context,
	identityManager identitymanagement.IdentityManagement,
) (string, error) {
	if m.userName != "" {
		return m.userName, nil
	}
	authCtx, err := cmkContext.ExtractClientDataAuthContext(ctx)
	if err != nil {
		return "", err
	}
	user, err := identityManager.GetUser(ctx, &identitymanagement.GetUserRequest{
		UserID:      m.UserID,
		AuthContext: identitymanagement.AuthContext{Data: authCtx},
	})
	if err != nil {
		return "", err
	}
	return user.User.Name, nil
}
