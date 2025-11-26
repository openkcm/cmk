package testutils

import "github.com/google/uuid"

type OrbitalJob struct {
	ID           uuid.UUID
	ExternalID   string
	Data         []byte
	Type         string
	Status       string
	ErrorMessage string
	UpdatedAt    int64
	CreatedAt    int64
}

func (OrbitalJob) TableName() string {
	return "jobs"
}

func (OrbitalJob) IsSharedModel() bool {
	return false
}
