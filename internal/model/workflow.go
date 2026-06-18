package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/utils/enums"
	"github.com/openkcm/cmk/utils/identity"
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

	ID                     uuid.UUID            `gorm:"type:uuid;primaryKey"`
	State                  WorkflowState        `gorm:"type:varchar(50);not null"`
	InitiatorID            string               `gorm:"type:varchar(255);not null"`
	initiatorName          string               `gorm:"-:all"`
	Approvers              []WorkflowApprover   `gorm:"foreignKey:WorkflowID"`
	ApproverGroupIDs       json.RawMessage      `gorm:"type:jsonb"`
	ArtifactType           WorkflowArtifactType `gorm:"type:varchar(50);not null"`
	ArtifactID             uuid.UUID            `gorm:"type:uuid;not null"`
	ArtifactName           *string              `gorm:"type:varchar(255)"` // Currently a snapshot at time of creation
	ActionType             WorkflowActionType   `gorm:"type:varchar(50);not null"`
	Parameters             string               `gorm:"type:text"`
	ParametersResourceName *string              `gorm:"type:varchar(255)"`
	ParametersResourceType *string              `gorm:"type:varchar(50)"`
	FailureReason          string               `gorm:"type:text"`
	ExpiryDate             *time.Time
	MinimumApprovalCount   int `gorm:"type:integer;default:2"` // Snapshot of minimum approvals at creation time
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
	case WorkflowArtifactTypeSystem:
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

	name, err := identity.GetUserName(ctx, identityManager, w.InitiatorID)
	if err != nil {
		return "", err
	}
	w.initiatorName = name
	return name, nil
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

	name, err := identity.GetUserName(ctx, identityManager, m.UserID)
	if err != nil {
		return "", err
	}
	m.userName = name
	return name, nil
}

var (
	ErrInvalidWorkflowState        = errors.New("invalid workflow state")
	ErrInvalidWorkflowArtifactType = errors.New("invalid workflow artifact type")
	ErrInvalidWorkflowActionType   = errors.New("invalid workflow action type")
)

//nolint:recvcheck
type WorkflowState string

//nolint:recvcheck
type WorkflowArtifactType string

//nolint:recvcheck
type WorkflowActionType string

const (
	WorkflowStateInitial          WorkflowState = "INITIAL"
	WorkflowStateRevoked          WorkflowState = "REVOKED"
	WorkflowStateRejected         WorkflowState = "REJECTED"
	WorkflowStateExpired          WorkflowState = "EXPIRED"
	WorkflowStateWaitApproval     WorkflowState = "WAIT_APPROVAL"
	WorkflowStateWaitConfirmation WorkflowState = "WAIT_CONFIRMATION"
	WorkflowStateExecuting        WorkflowState = "EXECUTING"
	WorkflowStateSuccessful       WorkflowState = "SUCCESSFUL"
	WorkflowStateFailed           WorkflowState = "FAILED"

	WorkflowArtifactTypeKey              WorkflowArtifactType = "KEY"
	WorkflowArtifactTypeKeyConfiguration WorkflowArtifactType = "KEY_CONFIGURATION"
	WorkflowArtifactTypeSystem           WorkflowArtifactType = "SYSTEM"

	WorkflowActionTypeUpdateState   WorkflowActionType = "UPDATE_STATE"
	WorkflowActionTypeUpdatePrimary WorkflowActionType = "UPDATE_PRIMARY"
	WorkflowActionTypeLink          WorkflowActionType = "LINK"
	WorkflowActionTypeUnlink        WorkflowActionType = "UNLINK"
	WorkflowActionTypeSwitch        WorkflowActionType = "SWITCH"
	WorkflowActionTypeDelete        WorkflowActionType = "DELETE"
)

var WorkflowStates = []WorkflowState{
	WorkflowStateInitial, WorkflowStateRevoked, WorkflowStateRejected, WorkflowStateExpired,
	WorkflowStateWaitApproval, WorkflowStateWaitConfirmation, WorkflowStateExecuting,
	WorkflowStateSuccessful, WorkflowStateFailed,
}

var WorkflowArtifactTypes = []WorkflowArtifactType{
	WorkflowArtifactTypeKey, WorkflowArtifactTypeKeyConfiguration, WorkflowArtifactTypeSystem,
}

var WorkflowActionTypes = []WorkflowActionType{
	WorkflowActionTypeUpdateState, WorkflowActionTypeUpdatePrimary,
	WorkflowActionTypeLink, WorkflowActionTypeUnlink, WorkflowActionTypeSwitch, WorkflowActionTypeDelete,
}

var WorkflowNonTerminalStates = []WorkflowState{
	WorkflowStateInitial, WorkflowStateWaitApproval, WorkflowStateWaitConfirmation, WorkflowStateExecuting,
}

var WorkflowTerminalStates = []WorkflowState{
	WorkflowStateRevoked, WorkflowStateRejected, WorkflowStateExpired, WorkflowStateSuccessful, WorkflowStateFailed,
}

func (s WorkflowState) String() string        { return string(s) }
func (t WorkflowArtifactType) String() string { return string(t) }
func (t WorkflowActionType) String() string   { return string(t) }

func (s WorkflowState) Valid() bool { return slices.Contains(WorkflowStates, s) }

func (s WorkflowState) Value() (driver.Value, error) {
	return enums.Value(s, ErrInvalidWorkflowState)
}

func (s *WorkflowState) Scan(src any) error {
	return enums.Scan(src, s, ErrInvalidWorkflowState)
}

func (t WorkflowArtifactType) Valid() bool {
	return slices.Contains(WorkflowArtifactTypes, t)
}

func (t WorkflowArtifactType) Value() (driver.Value, error) {
	return enums.Value(t, ErrInvalidWorkflowArtifactType)
}

func (t *WorkflowArtifactType) Scan(src any) error {
	return enums.Scan(src, t, ErrInvalidWorkflowArtifactType)
}

func (t WorkflowActionType) Valid() bool {
	return slices.Contains(WorkflowActionTypes, t)
}

func (t WorkflowActionType) Value() (driver.Value, error) {
	return enums.Value(t, ErrInvalidWorkflowActionType)
}

func (t *WorkflowActionType) Scan(src any) error {
	return enums.Scan(src, t, ErrInvalidWorkflowActionType)
}
