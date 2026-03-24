package testutils

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

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

func (OrbitalJob) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return true, nil
}
