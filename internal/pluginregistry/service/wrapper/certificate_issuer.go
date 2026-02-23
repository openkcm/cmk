package servicewrapper

import (
	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/certificate_issuer"
)

type certificateIssuerRepository struct {
	certificate_issuer.Repository
}

func (repo *certificateIssuerRepository) Binder() any {
	return repo.SetCertificateIssuer
}

func (repo *certificateIssuerRepository) Constraints() api.Constraints {
	return api.ExactlyOne()
}

func (repo *certificateIssuerRepository) Versions() []api.Version {
	return []api.Version{certificateIssuerV1{}}
}

type certificateIssuerV1 struct{}

func (certificateIssuerV1) New() api.Facade  { return new(certificate_issuer.V1) }
func (certificateIssuerV1) Deprecated() bool { return false }
