package clientcertificates

import (
	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

func transformTenantDefault(cc manager.ClientCertificate) (*cmkapi.TenantDefaultCertificate, error) {
	return &cmkapi.TenantDefaultCertificate{
		Name:    cc.Name,
		RootCA:  cc.RootCA,
		Subject: cc.Subject,
	}, nil
}

func transformCrypto(cc manager.ClientCertificate) (*cmkapi.CryptoCertificate, error) {
	return &cmkapi.CryptoCertificate{
		Name:    cc.Name,
		RootCA:  cc.RootCA,
		Subject: cc.Subject,
	}, nil
}

func ToAPI(cc map[model.CertificatePurpose][]*manager.ClientCertificate) (*cmkapi.ClientCertificates, error) {
	tenantDefaultCertList, err := transform.ToList(
		cc[model.CertificatePurposeTenantDefault],
		transformTenantDefault,
	)
	if err != nil {
		return nil, err
	}

	cryptoCertList, err := transform.ToList(
		cc[model.CertificatePurposeCrypto],
		transformCrypto,
	)
	if err != nil {
		return nil, err
	}

	return &cmkapi.ClientCertificates{
		TenantDefault: &cmkapi.TenantDefaultCertificateList{Count: ptr.PointTo(len(tenantDefaultCertList)),
			Value: tenantDefaultCertList},
		Crypto: &cmkapi.CryptoCertificateList{Count: ptr.PointTo(len(cryptoCertList)),
			Value: cryptoCertList},
	}, nil
}
