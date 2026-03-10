package tasks

import (
	"context"
	"crypto/rsa"
	"errors"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type CertUpdater interface {
	RotateExpiredCertificates(ctx context.Context) error
	RotateCertificate(ctx context.Context, args model.RequestCertArgs) (*model.Certificate,
		*rsa.PrivateKey, error)
}

type CertRotator struct {
	certClient CertUpdater
	repo       repo.Repo
	processor  *async.BatchProcessor
}

func NewCertRotator(
	certClient CertUpdater,
	repo repo.Repo,
) *CertRotator {
	return &CertRotator{
		certClient: certClient,
		repo:       repo,
		processor:  async.NewBatchProcessor(repo),
	}
}

var ErrRotatingCert = errors.New("error rotating certificate")

func (s *CertRotator) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Certificate Rotation Task")

	err := s.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			err := s.certClient.RotateExpiredCertificates(ctx)
			if err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		return s.handleErrorTenants(ctx, err)
	}

	return nil
}

func (s *CertRotator) TaskType() string {
	return config.TypeCertificateTask
}

func (s *CertRotator) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during certificate rotation batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}
