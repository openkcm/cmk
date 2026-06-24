package clientcertificates

import (
	"errors"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

var ErrInvalidCertificateSubject = errors.New("invalid certificate subject")

func transformCertificate(c model.ClientCertificate) (*cmkapi.Certificate, error) {
	if len(c.Subject.Country) < 1 || len(c.Subject.Locality) < 1 || len(c.Subject.Organization) < 1 {
		return nil, ErrInvalidCertificateSubject
	}
	return &cmkapi.Certificate{
		Name:   c.Name,
		RootCA: c.RootCA,
		Subject: cmkapi.CertificateSubject{
			C:  c.Subject.Country[0],
			CN: c.Subject.CommonName,
			L:  c.Subject.Locality[0],
			O:  c.Subject.Organization[0],
			OU: c.Subject.OrganizationalUnit,
		},
	}, nil
}

func ToAPI(cc model.ClientCertificates) (*cmkapi.ClientCertificates, error) {
	for _, v := range cc {
		err := sanitise.Sanitize(&v)
		if err != nil {
			return nil, err
		}
	}

	tenantDefaultCertList, err := transform.ToList(
		cc[model.CertificatePurposeHYOKManagement],
		transformCertificate,
	)
	if err != nil {
		return nil, err
	}

	cryptoCertList, err := transform.ToList(
		cc[model.CertificatePurposeCrypto],
		transformCertificate,
	)
	if err != nil {
		return nil, err
	}

	return &cmkapi.ClientCertificates{
		TenantDefault: &cmkapi.CertificateList{
			Count: ptr.PointTo(len(tenantDefaultCertList)),
			Value: tenantDefaultCertList,
		},
		Crypto: &cmkapi.CertificateList{
			Count: ptr.PointTo(len(cryptoCertList)),
			Value: cryptoCertList,
		},
	}, nil
}
