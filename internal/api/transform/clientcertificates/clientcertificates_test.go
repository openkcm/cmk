package clientcertificates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/clientcertificates"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    model.ClientCertificates
		expected *cmkapi.ClientCertificates
	}{
		{
			name: "Valid",
			input: model.ClientCertificates{
				model.CertificatePurposeHYOKManagement: {
					{
						RootCA: "TDRoot",
						Subject: model.CertificateSubject{
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
						Subject: model.CertificateSubject{
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
				TenantDefault: &cmkapi.CertificateList{
					Count: ptr.PointTo(1),
					Value: []cmkapi.Certificate{{
						RootCA: "TDRoot",
						Subject: cmkapi.CertificateSubject{
							C:  "C",
							CN: "CN",
							L:  "L",
							O:  "O",
							OU: []string{"OU1", "OU2"},
						},
					}},
				},
				Crypto: &cmkapi.CertificateList{
					Count: ptr.PointTo(1),
					Value: []cmkapi.Certificate{{
						RootCA: "CRoot",
						Subject: cmkapi.CertificateSubject{
							C:  "C",
							CN: "CN",
							L:  "L",
							O:  "O",
							OU: []string{"OU1", "OU2"},
						},
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
