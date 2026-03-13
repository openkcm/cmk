package tasks

import (
	"context"
	"crypto/rsa"
	"errors"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
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
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	c := &CertRotator{
		certClient: certClient,
		repo:       repo,
		processor:  async.NewBatchProcessor(repo),
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

var ErrRotatingCert = errors.New("error rotating certificate")

func (c *CertRotator) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Certificate Rotation Task")

	err := c.certClient.RotateExpiredCertificates(ctx)
	if err != nil {
		return c.handleErrorTenants(ctx, err)
	}

	return nil
}

func (c *CertRotator) TaskType() string {
	return config.TypeCertificateTask
}

func (c *CertRotator) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (c *CertRotator) FanOutFunc() async.FunOutFunc {
	return async.TenantFanOut
}

func (c *CertRotator) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during certificate rotation batch processing", err)
	return err
}
