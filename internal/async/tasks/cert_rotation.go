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
	fanout     bool
}

func NewCertRotator(
	certClient CertUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) *CertRotator {
	c := &CertRotator{
		certClient: certClient,
		repo:       repo,
		processor:  async.NewBatchProcessor(repo),
	}

	for _, o := range opts {
		o(c)
	}

	log.Debug(context.Background(), "Created Cert Rotation Task")

	return c
}

var ErrRotatingCert = errors.New("error rotating certificate")

func (s *CertRotator) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Certificate Rotation Task")

	if async.IsChildTask(task) {
		return async.ProcessChildTask(ctx, task, s.process)
	}

	err := s.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, _ *model.Tenant) error {
			return s.process(ctx)
		},
	)
	if err != nil {
		return s.handleErrorTenants(ctx, err)
	}

	return nil
}

func (s *CertRotator) process(ctx context.Context) error {
	err := s.certClient.RotateExpiredCertificates(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *CertRotator) TaskType() string {
	return config.TypeCertificateTask
}

func (s *CertRotator) SetFanOut(client async.Client) {
	s.processor = async.NewBatchProcessor(s.repo, async.WithFanOutTenants(client))
	s.fanout = true
}

func (s *CertRotator) IsFanOutEnabled() bool {
	return s.fanout
}

func (s *CertRotator) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during certificate rotation batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}
