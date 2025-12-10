package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const WorkflowID = "workflow_id"

// Workflow is an action on a data model (artifact) and can be read as <Artifact><ActionType>
// Artifact type is the type of item, identified by ArtifactID and ActionType the executed action
// Parameters will have different values depending on the ActionType.
// Check API Yaml for possible Parameters
//
// e.g. of a workflow
// System Link
type Workflow struct {
	AutoTimeModel

	ID            uuid.UUID          `gorm:"type:uuid;primaryKey"`
	State         string             `gorm:"type:varchar(50);not null"`
	InitiatorID   string             `gorm:"type:varchar(255);not null"`
	InitiatorName string             `gorm:"type:varchar(255);not null"`
	Approvers     []WorkflowApprover `gorm:"foreignKey:WorkflowID"`
	ArtifactType  string             `gorm:"type:varchar(50);not null"`
	ArtifactID    uuid.UUID          `gorm:"type:uuid;not null"`
	ActionType    string             `gorm:"type:varchar(50);not null"`
	Parameters    string             `gorm:"type:text"`
	FailureReason string             `gorm:"type:text"`
	ExpiryDate    time.Time          `gorm:"not null"`
}

func (w Workflow) TableName() string   { return "workflows" }
func (w Workflow) IsSharedModel() bool { return false }

func (w Workflow) BeforeDelete(tx *gorm.DB) error {
	// Delete all associated workflow approvers
	return tx.Where(WorkflowID+" = ?", w.ID).Delete(&WorkflowApprover{}).Error
}

func (w Workflow) ApproverIDs() []string {
	ids := make([]string, len(w.Approvers))
	for i, approver := range w.Approvers {
		ids[i] = approver.UserID
	}

	return ids
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
