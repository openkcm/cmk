package tasks_test

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

type CertUpdaterMock struct{}

func (s *CertUpdaterMock) GetCertificatesForRotation(_ context.Context,
) ([]*model.Certificate, error) {
	return []*model.Certificate{}, nil
}

func (s *CertUpdaterMock) RotateCertificate(_ context.Context,
	_ model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

func TestCertificateRotatorProcessAction(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	repo := sql.NewRepository(db)

	rotator := tasks.NewCertRotator(&CertUpdaterMock{}, repo)

	t.Run("Should Create", func(t *testing.T) {
		err := rotator.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
	})

	t.Run("Should Create with tenant lists", func(t *testing.T) {
		payload := asyncUtils.NewTenantListPayload([]string{"tenant1", "tenant2"})
		payloadBytes, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask("", payloadBytes)
		err = rotator.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
	})
}
