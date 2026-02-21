package certificate_issuer

import (
	"errors"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
)

var ErrNotConfigured = errors.New("certificate issuer plugin not configured")

type Repository struct {
	Instance certificateissuer.CertificateIssuer
}

func (repo *Repository) CertificateIssuer() (certificateissuer.CertificateIssuer, error) {
	if repo.Instance == nil {
		return nil, ErrNotConfigured
	}
	return repo.Instance, nil
}

func (repo *Repository) SetCertificateIssuer(instance certificateissuer.CertificateIssuer) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
