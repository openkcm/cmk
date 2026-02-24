package certificateissuer

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"
)

type CertificateIssuer interface {
	ServiceInfo() api.Info

	IssueCertificate(ctx context.Context, req *IssueCertificateRequest) (*IssueCertificateResponse, error)
}

type ValidityType int32

const (
	Unspecified ValidityType = iota
	Days
	Months
	Years
)

type IssueCertificateRequest struct {
	// V1 Fields
	CommonName string
	Localities []string
	Validity   *CertificateValidity
	PrivateKey *CertificatePrivateKey
}

type IssueCertificateResponse struct {
	// V1 Fields
	ChainPem string
}

type CertificateValidity struct {
	// V1 Fields
	Value int64
	Type  ValidityType
}

type CertificatePrivateKey struct {
	// V1 Fields
	Data []byte
}
