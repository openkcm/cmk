package tasks_test

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

type CertUpdaterMock struct{}

func (s *CertUpdaterMock) RotateExpiredCertificates(_ context.Context) error {
	return nil
}

func (s *CertUpdaterMock) RotateCertificate(_ context.Context,
	_ model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

func TestCertificateRotatorProcessAction(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	rotator := tasks.NewCertRotator(&CertUpdaterMock{}, r)

	t.Run("Should Create", func(t *testing.T) {
		task := asynq.NewTask(config.TypeCertificateTask, nil)
		err := rotator.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
	})

	t.Run("Should Create with tenant lists", func(t *testing.T) {
		payload := asyncUtils.NewTenantListPayload([]string{"tenant1", "tenant2"})
		payloadBytes, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeCertificateTask, payloadBytes)
		err = rotator.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeCertificateTask, rotator.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), rotator.TenantQuery())
	})
}
