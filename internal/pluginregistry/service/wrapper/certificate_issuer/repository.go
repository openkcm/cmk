package certificate_issuer

import (
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
)

type Repository struct {
	Instance certificateissuer.CertificateIssuer
}

func (repo *Repository) CertificateIssuer() (certificateissuer.CertificateIssuer, bool) {
	return repo.Instance, repo.Instance != nil
}

func (repo *Repository) SetCertificateIssuer(instance certificateissuer.CertificateIssuer) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
