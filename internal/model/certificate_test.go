package model_test

import (
	"crypto/x509/pkix"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
)

func TestCertificateTable(t *testing.T) {
	t.Run("Should have table name certificates", func(t *testing.T) {
		expectedTableName := "certificates"

		tableName := model.Certificate{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Certificate{}.IsSharedModel())
	})
}

func TestFormatSubjectWithSlashSeparatedOUs(t *testing.T) {
	tests := []struct {
		name     string
		subject  model.CertificateSubject
		expected string
	}{
		{
			name: "single OU uses standard format",
			subject: model.CertificateSubject{
				CommonName:         "test-tenant",
				Country:            []string{"DE"},
				Organization:       []string{"TestOrg"},
				OrganizationalUnit: []string{"OU1"},
				Locality:           []string{"Berlin"},
			},
			expected: "CN=test-tenant,OU=OU1,O=TestOrg,L=Berlin,C=DE",
		},
		{
			name: "multiple OUs joined with slash",
			subject: model.CertificateSubject{
				CommonName:         "test-tenant",
				Country:            []string{"DE"},
				Organization:       []string{"TestOrg"},
				OrganizationalUnit: []string{"OU1", "OU2"},
				Locality:           []string{"Berlin"},
			},
			expected: "CN=test-tenant,OU=OU1/OU2,O=TestOrg,L=Berlin,C=DE",
		},
		{
			name: "no OU",
			subject: model.CertificateSubject{
				CommonName:   "test-tenant",
				Country:      []string{"DE"},
				Organization: []string{"TestOrg"},
			},
			expected: "CN=test-tenant,O=TestOrg,C=DE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.subject.String())
		})
	}
}

func TestNewClientCertificateFromConfig(t *testing.T) {
	cfg := config.CryptoCert{
		Name:   "test-cert",
		RootCA: "https://example.com/root.crt",
		Subject: config.CryptoCertSubject{
			CommonNamePrefix:   "prefix_",
			Country:            []string{"DE"},
			Organization:       []string{"TestOrg"},
			OrganizationalUnit: []string{"OU1"},
			Locality:           []string{"Berlin"},
		},
	}

	cert := model.NewClientCertificate(cfg, "tenant-123")

	assert.Equal(t, "test-cert", cert.Name)
	assert.Equal(t, "https://example.com/root.crt", cert.RootCA)
	assert.Equal(t, "prefix_tenant-123", cert.Subject.CommonName)
	assert.Equal(t, []string{"DE"}, cert.Subject.Country)
	assert.Equal(t, []string{"TestOrg"}, cert.Subject.Organization)
	assert.Equal(t, []string{"OU1"}, cert.Subject.OrganizationalUnit)
	assert.Equal(t, []string{"Berlin"}, cert.Subject.Locality)
}

func TestNewClientCertificateSubjectFromPKIX(t *testing.T) {
	pkixName := pkix.Name{
		CommonName:         "tenant-cn",
		Country:            []string{"US"},
		Organization:       []string{"Org"},
		OrganizationalUnit: []string{"Unit"},
		Locality:           []string{"NYC"},
	}

	subject := model.ToCertificateSubjectFromPKIX(pkixName)

	assert.Equal(t, "tenant-cn", subject.CommonName)
	assert.Equal(t, []string{"US"}, subject.Country)
	assert.Equal(t, []string{"Org"}, subject.Organization)
	assert.Equal(t, []string{"Unit"}, subject.OrganizationalUnit)
	assert.Equal(t, []string{"NYC"}, subject.Locality)
}
