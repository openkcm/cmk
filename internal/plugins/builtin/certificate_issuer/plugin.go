package certificate_issuer

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	certificate_issuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
	slogctx "github.com/veqryn/slog-context"
)

const (
	pluginName = "certificate-issuer-empty"
)

func V1BuiltIn() catalog.BuiltIn {
	return builtin(&V1Plugin{})
}

func builtin(p *V1Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		certificate_issuerv1.CertificateIssuerServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	certificate_issuerv1.UnimplementedCertificateIssuerServiceServer
}

var (
	_ certificate_issuerv1.CertificateIssuerServiceServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer                               = (*V1Plugin)(nil)
)

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(ctx context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slogctx.Debug(ctx, "Builtin Certificate Issuer Service (cis) plugin")

	return &configv1.ConfigureResponse{}, nil
}

// GetCertificate V1Plugin method/operation
func (p *V1Plugin) GetCertificate(ctx context.Context, _ *certificate_issuerv1.GetCertificateRequest) (*certificate_issuerv1.GetCertificateResponse, error) {

	slogctx.Debug(ctx, "Builtin ertificate Issuer Service (cis) - GetCertificate called")

	return &certificate_issuerv1.GetCertificateResponse{
		CertificateChain: "",
	}, nil
}
