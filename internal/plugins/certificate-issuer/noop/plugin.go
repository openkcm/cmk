package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	certificateissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.MakeBuiltIn("noop",
		certificateissuerv1.CertificateIssuerServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type Plugin struct {
	certificateissuerv1.UnsafeCertificateIssuerServiceServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string
}

var (
	_ certificateissuerv1.CertificateIssuerServiceServer = (*Plugin)(nil)
	_ configv1.ConfigServer                              = (*Plugin)(nil)
)

func NewPlugin() *Plugin {
	return &Plugin{
		buildInfo: "{}",
	}
}

func (p *Plugin) SetLogger(logger hclog.Logger) {
	p.logger = hclog2slog.New(logger)
}

func (p *Plugin) Configure(_ context.Context, req *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{
		BuildInfo: &p.buildInfo,
	}, nil
}

func (p *Plugin) GetCertificate(
	_ context.Context,
	_ *certificateissuerv1.GetCertificateRequest,
) (*certificateissuerv1.GetCertificateResponse, error) {
	return &certificateissuerv1.GetCertificateResponse{}, nil
}
