package tasks

import (
	"context"
	"crypto/rsa"
	"errors"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type CertUpdater interface {
	GetCertificatesForRotation(ctx context.Context,
	) ([]*model.Certificate, int, error)
	RotateCertificate(ctx context.Context, args model.RequestCertArgs) (*model.Certificate,
		*rsa.PrivateKey, error)
}

type CertRotator struct {
	certClient CertUpdater
	repo       repo.Repo
	processor  *BatchProcessor
}

func NewCertRotator(
	certClient CertUpdater,
	repo repo.Repo,
) *CertRotator {
	return &CertRotator{
		certClient: certClient,
		repo:       repo,
		processor:  NewBatchProcessor(repo),
	}
}

var ErrRotatingCert = errors.New("error rotating certificate")

func (s *CertRotator) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Certificate Rotation Task")

	err := s.processor.ProcessTenantsInBatch(ctx, "Certificate Rotation", task,
		func(tenantCtx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(tenantCtx, "Rotating Certificates for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			certs, _, certErr := s.certClient.GetCertificatesForRotation(tenantCtx)
			if certErr != nil {
				return s.handleErrorTask(tenantCtx, certErr)
			}

			for _, cert := range certs {
				certErr = s.handleCertificate(tenantCtx, cert, tenant.ID)
				if certErr != nil {
					return s.handleErrorTask(tenantCtx, certErr)
				}
			}
			log.Debug(tenantCtx, "Certificates for tenant are up-to-date", slog.String("schemaName", tenant.SchemaName))
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

func (s *CertRotator) handleCertificate(ctx context.Context, cert *model.Certificate,
	schemaName string,
) error {
	_, _, err := s.certClient.RotateCertificate(ctx, model.RequestCertArgs{
		CertPurpose: cert.Purpose,
		Supersedes:  &cert.ID,
		CommonName:  cert.CommonName,
		Locality:    []string{schemaName},
	})
	if err != nil {
		log.Error(ctx, "Rotating Certificate", err)

		return errs.Wrap(ErrRunningTask, ErrRotatingCert)
	}

	return nil
}

func (s *CertRotator) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during certificate rotation batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (s *CertRotator) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running Cert Refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}
