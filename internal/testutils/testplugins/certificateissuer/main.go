package main

import (
	"context"

	"github.com/openkcm/plugin-sdk/pkg/plugin"

	certificate_issuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type TestPlugin struct {
	configv1.UnsafeConfigServer
	certificate_issuerv1.UnimplementedCertificateIssuerServiceServer
}

var _ certificate_issuerv1.CertificateIssuerServiceServer = (*TestPlugin)(nil)

func (p *TestPlugin) Configure(_ context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	return &configv1.ConfigureResponse{}, nil
}

func (p *TestPlugin) GetCertificate(_ context.Context, _ *certificate_issuerv1.GetCertificateRequest) (
	*certificate_issuerv1.GetCertificateResponse, error,
) {
	return &certificate_issuerv1.GetCertificateResponse{}, nil
}

func New() *TestPlugin {
	return &TestPlugin{}
}

func main() {
	server := New()

	plugin.Serve(certificate_issuerv1.CertificateIssuerServicePluginServer(server),
		configv1.ConfigServiceServer(server))
}
