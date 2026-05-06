package testplugins

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

type TestCertificateIssuer struct{}

var _ certificateissuer.CertificateIssuer = (*TestCertificateIssuer)(nil)

func NewTestCertificateIssuer() *TestCertificateIssuer {
	return &TestCertificateIssuer{}
}

func (s *TestCertificateIssuer) ServiceInfo() api.Info {
	return testInfo{
		configuredType: servicewrapper.CertificateIssuerServiceType,
	}
}

func (s *TestCertificateIssuer) IssueCertificate(
	_ context.Context,
	_ *certificateissuer.IssueCertificateRequest,
) (*certificateissuer.IssueCertificateResponse, error) {
	return &certificateissuer.IssueCertificateResponse{}, nil
}
