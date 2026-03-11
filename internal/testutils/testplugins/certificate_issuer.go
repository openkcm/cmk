package testplugins

import (
	"context"
	"log/slog"

	"github.com/openkcm/plugin-sdk/pkg/catalog"

	certificateissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type CertificateIssuer struct {
	certificateissuerv1.UnsafeCertificateIssuerServiceServer
	configv1.UnsafeConfigServer
}

func NewCertificateIssuer() catalog.BuiltInPlugin {
	p := &CertificateIssuer{}
	return catalog.MakeBuiltIn(
		Name,
		certificateissuerv1.CertificateIssuerServicePluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}

func (p *CertificateIssuer) Configure(
	_ context.Context,
	req *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *CertificateIssuer) GetCertificate(
	_ context.Context,
	_ *certificateissuerv1.GetCertificateRequest,
) (*certificateissuerv1.GetCertificateResponse, error) {
	return &certificateissuerv1.GetCertificateResponse{}, nil
}
