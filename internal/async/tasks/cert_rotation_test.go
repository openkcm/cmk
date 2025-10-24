package tasks_test

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

type CertUpdaterMock struct{}

func (s *CertUpdaterMock) GetCertificatesForRotation(_ context.Context,
) ([]*model.Certificate, int, error) {
	return []*model.Certificate{}, 0, nil
}

func (s *CertUpdaterMock) RotateCertificate(_ context.Context,
	_ manager.RequestNewCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

func TestCertificateRotatorProcessAction(t *testing.T) {
	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&testutils.TestModel{}},
	})
	repo := sql.NewRepository(db)

	rotator := tasks.NewCertRotator(&CertUpdaterMock{}, repo)

	t.Run("Should Create", func(t *testing.T) {
		err := rotator.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
	})
}
