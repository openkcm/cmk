package clientcertificates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/clientcertificates"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    map[model.CertificatePurpose][]*manager.ClientCertificate
		expected *cmkapi.ClientCertificates
	}{
		{
			name: "Valid",
			input: map[model.CertificatePurpose][]*manager.ClientCertificate{
				model.CertificatePurposeTenantDefault: {
					{
						RootCA: "TDRoot",
						Subject: manager.ClientCertificateSubject{
							Locality:           []string{"L"},
							OrganizationalUnit: []string{"OU1", "OU2"},
							Organization:       []string{"O"},
							Country:            []string{"C"},
							CommonName:         "CN",
						},
					},
				},
				model.CertificatePurposeCrypto: {
					{
						RootCA: "CRoot",
						Subject: manager.ClientCertificateSubject{
							Locality:           []string{"L"},
							OrganizationalUnit: []string{"OU1", "OU2"},
							Organization:       []string{"O"},
							Country:            []string{"C"},
							CommonName:         "CN",
						},
					},
				},
			},
			expected: &cmkapi.ClientCertificates{
				TenantDefault: &cmkapi.TenantDefaultCertificateList{
					Count: ptr.PointTo(1),
					Value: []cmkapi.TenantDefaultCertificate{{
						RootCA:  "TDRoot",
						Subject: "CN=CN,OU=OU1/OU2,O=O,L=L,C=C",
					}},
				},
				Crypto: &cmkapi.CryptoCertificateList{
					Count: ptr.PointTo(1),
					Value: []cmkapi.CryptoCertificate{{
						RootCA:  "CRoot",
						Subject: "CN=CN,OU=OU1/OU2,O=O,L=L,C=C",
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := clientcertificates.ToAPI(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.NoError(t, err)
		})
	}
}
