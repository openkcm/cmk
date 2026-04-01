package clientcertificates

import (
	"crypto/x509/pkix"
	"regexp"
	"strings"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

// formatSubjectWithSlashSeparatedOUs transforms the standard X.509 subject string
// to combine multiple OUs with / separator instead of +
func formatSubjectWithSlashSeparatedOUs(subject manager.ClientCertificateSubject) string {
	s := pkix.Name{
		Locality:           subject.Locality,
		Country:            subject.Country,
		Organization:       subject.Organization,
		OrganizationalUnit: subject.OrganizationalUnit,
		CommonName:         subject.CommonName,
	}
	if len(s.OrganizationalUnit) <= 1 {
		return s.String() // Use standard format if 0 or 1 OU
	}

	// Get standard format
	standardSubject := s.String()

	// Replace OU=X+OU=Y+OU=Z with OU=X/Y/Z
	combinedOU := "OU=" + strings.Join(s.OrganizationalUnit, "/")

	// Build regex to match multiple OU entries
	ouPattern := `OU=[^,+]+((\+OU=[^,+]+)+)`
	re := regexp.MustCompile(ouPattern)

	return re.ReplaceAllString(standardSubject, combinedOU)
}

func transformTenantDefault(cc manager.ClientCertificate) (*cmkapi.TenantDefaultCertificate, error) {
	return &cmkapi.TenantDefaultCertificate{
		Name:    cc.Name,
		RootCA:  cc.RootCA,
		Subject: formatSubjectWithSlashSeparatedOUs(cc.Subject),
	}, nil
}

func transformCrypto(cc manager.ClientCertificate) (*cmkapi.CryptoCertificate, error) {
	return &cmkapi.CryptoCertificate{
		Name:    cc.Name,
		RootCA:  cc.RootCA,
		Subject: formatSubjectWithSlashSeparatedOUs(cc.Subject),
	}, nil
}

func ToAPI(cc map[model.CertificatePurpose][]*manager.ClientCertificate) (*cmkapi.ClientCertificates, error) {
	for _, v := range cc {
		err := sanitise.Sanitize(&v)
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
