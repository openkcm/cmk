package clientcertificates

import (
	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/transform"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/ptr"
	"github.tools.sap/kms/cmk/utils/sanitise"
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
	for _, v := range cc {
		err := sanitise.Stringlikes(&v)
		if err != nil {
			return nil, err
		}
	}

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
