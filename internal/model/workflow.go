package model

import (
	"database/sql"

	"github.com/google/uuid"
)

const WorkflowID = "workflow_id"

type Workflow struct {
	AutoTimeModel

	ID            uuid.UUID          `gorm:"type:uuid;primaryKey"`
	State         string             `gorm:"type:varchar(50);not null"`
	InitiatorID   uuid.UUID          `gorm:"type:uuid;not null"`
	InitiatorName string             `gorm:"type:varchar(255);not null"`
	Approvers     []WorkflowApprover `gorm:"foreignKey:WorkflowID"`
	ArtifactType  string             `gorm:"type:varchar(50);not null"`
	ArtifactID    uuid.UUID          `gorm:"type:uuid;not null"`
	ActionType    string             `gorm:"type:varchar(50);not null"`
	Parameters    string             `gorm:"type:text"`
	FailureReason string             `gorm:"type:text"`
}

func (w Workflow) TableName() string   { return "workflows" }
func (w Workflow) IsSharedModel() bool { return false }

func (w Workflow) ApproverIDs() []uuid.UUID {
	ids := make([]uuid.UUID, len(w.Approvers))
	for i, approver := range w.Approvers {
		ids[i] = approver.UserID
	}

	return ids
}

type WorkflowApprover struct {
	WorkflowID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserName   string    `gorm:"type:varchar(255);not null"`

	Workflow Workflow     `gorm:"foreignKey:WorkflowID"`
	Approved sql.NullBool `gorm:"default:null"`
}

func (w WorkflowApprover) TableName() string   { return "workflow_approvers" }
func (w WorkflowApprover) IsSharedModel() bool { return false }
