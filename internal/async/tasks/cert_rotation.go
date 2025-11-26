package tasks

import (
	"context"
	"crypto/rsa"
	"errors"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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
}

func NewCertRotator(
	certClient CertUpdater,
	repo repo.Repo,
) *CertRotator {
	return &CertRotator{
		certClient: certClient,
		repo:       repo,
	}
}

var (
	ErrDecodingPK      = errors.New("error decoding private key")
	ErrParsingPK       = errors.New("error parsing private key")
	ErrRotatingCert    = errors.New("error rotating certificate")
	ErrUpdatingOldCert = errors.New("error updating old certificate")
)

func (s *CertRotator) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	var tenants []*model.Tenant

	_, err := s.repo.List(ctx, model.Tenant{}, &tenants, *repo.NewQuery())
	if err != nil {
		return s.handleErrorTenants(ctx, err)
	}

	for _, tenant := range tenants {
		ctx := log.InjectTenant(cmkcontext.CreateTenantContext(ctx, tenant.ID), tenant)

		certs, _, err := s.certClient.GetCertificatesForRotation(ctx)
		if err != nil {
			return s.handleErrorTask(ctx, err)
		}

		for _, cert := range certs {
			err = s.handleCertificate(ctx, cert, tenant.ID)
			if err != nil {
				return s.handleErrorTask(ctx, err)
			}
		}
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
	log.Error(ctx, "Getting Tenants on Cert Refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (s *CertRotator) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running Cert Refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}
