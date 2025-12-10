package testutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/testutils"
)

func TestCreateTestCertificate(t *testing.T) {
	tests := []struct {
		name     string
		mutator  func(*model.Certificate)
		validate func(*testing.T, *model.Certificate)
	}{
		{
			name:    "Default Certificate",
			mutator: func(_ *model.Certificate) {},
			validate: func(t *testing.T, cert *model.Certificate) {
				t.Helper()
				assert.Equal(t, model.CertificatePurposeTenantDefault, cert.Purpose)
				assert.Equal(t, manager.DefaultHYOKCertCommonName, cert.CommonName)
				assert.Equal(t, model.CertificateStateActive, cert.State)
				assert.NotEmpty(t, cert.CertPEM)
				assert.NotEmpty(t, cert.PrivateKeyPEM)
			},
		},
		{
			name: "Custom Certificate Fingerprint",
			mutator: func(cert *model.Certificate) {
				cert.Fingerprint = "custom-fingerprint"
			},
			validate: func(t *testing.T, cert *model.Certificate) {
				t.Helper()
				assert.Equal(t, "custom-fingerprint", cert.Fingerprint)
				assert.Equal(t, model.CertificatePurposeTenantDefault, cert.Purpose)
				assert.Equal(t, manager.DefaultHYOKCertCommonName, cert.CommonName)
				assert.Equal(t, model.CertificateStateActive, cert.State)
				assert.NotEmpty(t, cert.CertPEM)
				assert.NotEmpty(t, cert.PrivateKeyPEM)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := testutils.NewCertificate(tt.mutator)
			tt.validate(t, cert)
		})
	}
}
