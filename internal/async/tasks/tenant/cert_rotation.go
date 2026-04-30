package tasks

import (
	"context"
	"crypto/rsa"
	"errors"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type CertUpdater interface {
	RotateExpiredCertificates(ctx context.Context) error
	RotateCertificate(ctx context.Context, args model.RequestCertArgs) (*model.Certificate,
		*rsa.PrivateKey, error)
}

type CertRotator struct {
	certClient CertUpdater
	repo       repo.Repo
}

func NewCertRotator(
	certClient CertUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	c := &CertRotator{
		certClient: certClient,
		repo:       repo,
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

var ErrRotatingCert = errors.New("error rotating certificate")

func (c *CertRotator) ProcessTask(ctx context.Context, task *asynq.Task) error {
	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskCertRotationRole)
	if err != nil {
		c.logError(ctx, err)
		return nil
	}

	err = c.certClient.RotateExpiredCertificates(ctx)
	if err != nil {
		c.logError(ctx, err)
	}
	return nil
}

func (c *CertRotator) TaskType() string {
	return config.TypeCertificateTask
}

func (c *CertRotator) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (c *CertRotator) FanOutFunc() async.FanOutFunc {
	return async.TenantFanOut
}

func (c *CertRotator) logError(ctx context.Context, err error) {
	// Returned errors are retries in batch processor
	// If we don't want a retry we just log here and return nil
	log.Error(ctx, "Error during certificate rotation batch processing", err)
}
