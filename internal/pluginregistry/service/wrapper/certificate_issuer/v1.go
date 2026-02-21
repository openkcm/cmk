package certificate_issuer

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/plugin"

	grpccertificateissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
)

type V1 struct {
	plugin.Facade
	grpccertificateissuerv1.CertificateIssuerServicePluginClient
}

func (v1 *V1) Version() uint {
	return 1
}

func (v1 *V1) ServiceInfo() api.Info {
	return v1.Info
}

func (v1 *V1) IssueCertificate(
	ctx context.Context,
	req *certificateissuer.IssueCertificateRequest,
) (*certificateissuer.IssueCertificateResponse, error) {
	in := &grpccertificateissuerv1.GetCertificateRequest{
		CommonName: req.CommonName,
		Locality:   req.Localities,
		Validity:   CertificateValidityToGRPC(req.Validity),
		PrivateKey: CertificatePrivateKeyToGRPC(req.PrivateKey),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf("failed validation: %w", err)
	}

	grpcResp, err := v1.GetCertificate(ctx, in)
	if err != nil {
		return nil, err
	}
	return &certificateissuer.IssueCertificateResponse{
		ChainPem: grpcResp.GetCertificateChain(),
	}, nil
}

func CertificateValidityToGRPC(
	v *certificateissuer.CertificateValidity,
) *grpccertificateissuerv1.GetCertificateValidity {
	if v == nil {
		return nil
	}
	return &grpccertificateissuerv1.GetCertificateValidity{
		Value: v.Value,
		Type:  grpccertificateissuerv1.ValidityType(v.Type),
	}
}

func CertificatePrivateKeyToGRPC(pk *certificateissuer.CertificatePrivateKey) *grpccertificateissuerv1.PrivateKey {
	if pk == nil {
		return nil
	}
	return &grpccertificateissuerv1.PrivateKey{
		Data: pk.Data,
	}
}
