package clientcertificates

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

func transformTenantDefault(cc model.ClientCertificate) (*cmkapi.TenantDefaultCertificate, error) {
	return &cmkapi.TenantDefaultCertificate{
		Name:    cc.Name,
		RootCA:  cc.RootCA,
		Subject: cc.Subject.FormatSubjectWithSlashSeparatedOUs(),
	}, nil
}

func transformCrypto(cc model.ClientCertificate) (*cmkapi.CryptoCertificate, error) {
	return &cmkapi.CryptoCertificate{
		Name:    cc.Name,
		RootCA:  cc.RootCA,
		Subject: cc.Subject.FormatSubjectWithSlashSeparatedOUs(),
	}, nil
}

func ToAPI(cc map[model.CertificatePurpose][]*model.ClientCertificate) (*cmkapi.ClientCertificates, error) {
	for _, v := range cc {
		err := sanitise.Sanitize(&v)
		if err != nil {
			return nil, err
		}
	}

	tenantDefaultCertList, err := transform.ToList(
		cc[model.CertificatePurposeHYOKManagement],
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
		TenantDefault: &cmkapi.TenantDefaultCertificateList{
			Count: ptr.PointTo(len(tenantDefaultCertList)),
			Value: tenantDefaultCertList,
		},
		Crypto: &cmkapi.CryptoCertificateList{
			Count: ptr.PointTo(len(cryptoCertList)),
			Value: cryptoCertList,
		},
	}, nil
}
