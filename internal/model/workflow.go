package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/constants"
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
	InitiatorName          string             `gorm:"type:varchar(255);not null"`
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

func (w Workflow) TableName() string   { return "workflows" }
func (w Workflow) IsSharedModel() bool { return false }

func (w Workflow) BeforeDelete(tx *gorm.DB) error {
	// Delete all associated workflow approvers
	return tx.Where(WorkflowID+" = ?", w.ID).Delete(&WorkflowApprover{}).Error
}

func (w *Workflow) BeforeSave(tx *gorm.DB) error {
	if w.ExpiryDate == nil {
		w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, constants.DefaultExpiryPeriodDays))
	}
	return nil
}

// Description generates a human-readable description of the workflow based on its action type
func (w Workflow) Description() string {
	// Build workflow description based on artifact type first, then action type
	var description string

	switch w.ArtifactType {
	case constants.WorkflowArtifactTypeSystem:
		description = w.buildSystemDescription()
	default:
		description = w.buildDefaultDescription()
	}

	description += "."

	return description
}

// GetArtifactName returns the artifact name or a default value if nil
func (w Workflow) GetArtifactName() string {
	if w.ArtifactName != nil {
		return *w.ArtifactName
	}
	return ""
}

// buildSystemDescription generates a description for SYSTEM artifact workflows
func (w Workflow) buildSystemDescription() string {
	var description string
	artifactName := w.GetArtifactName()

	description = fmt.Sprintf("%s requested approval to %s %s",
		w.InitiatorName,
		w.ActionType,
		w.ArtifactType,
	)

	if artifactName != "" {
		description += fmt.Sprintf(": '%s'", artifactName)
	}

	if w.Parameters != "" {
		resourceType := w.getParametersResourceType()
		resourceName := w.getParametersResourceName()
		if resourceType != "" && resourceName != "" {
			description += fmt.Sprintf(" to %s: '%s'", resourceType, resourceName)
		}
	}

	return description
}

// getParametersResourceType returns the parameters resource type or empty string if nil
func (w Workflow) getParametersResourceType() string {
	if w.ParametersResourceType != nil {
		return *w.ParametersResourceType
	}
	return ""
}

// getParametersResourceName returns the parameters resource name or empty string if nil
func (w Workflow) getParametersResourceName() string {
	if w.ParametersResourceName != nil {
		return *w.ParametersResourceName
	}
	return ""
}

// buildDefaultDescription generates a default description for workflows
func (w Workflow) buildDefaultDescription() string {
	artifactName := w.GetArtifactName()
	description := fmt.Sprintf("%s requested approval to %s %s",
		w.InitiatorName,
		w.ActionType,
		w.ArtifactType,
	)

	if artifactName != "" {
		description += fmt.Sprintf(": '%s'", artifactName)
	}

	if w.Parameters != "" {
		description += " with parameters: " + w.Parameters
	}

	return description
}

type WorkflowApprover struct {
	WorkflowID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     string    `gorm:"type:varchar(255);primaryKey"`
	UserName   string    `gorm:"type:varchar(255);not null"`

	Workflow Workflow     `gorm:"foreignKey:WorkflowID"`
	Approved sql.NullBool `gorm:"default:null"`
}

func (w WorkflowApprover) TableName() string   { return "workflow_approvers" }
func (w WorkflowApprover) IsSharedModel() bool { return false }
