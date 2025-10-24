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
	}{{
		name: "Valid",
		input: map[model.CertificatePurpose][]*manager.ClientCertificate{
			model.CertificatePurposeTenantDefault: {
				{
					RootCA:  "TDRoot",
					Subject: "TDSub"}},
			model.CertificatePurposeCrypto: {
				{
					RootCA:  "CRoot",
					Subject: "CSub"}},
		},
		expected: &cmkapi.ClientCertificates{
			TenantDefault: &cmkapi.TenantDefaultCertificateList{
				Count: ptr.PointTo(1),
				Value: []cmkapi.TenantDefaultCertificate{{
					RootCA:  "TDRoot",
					Subject: "TDSub"}}},
			Crypto: &cmkapi.CryptoCertificateList{
				Count: ptr.PointTo(1),
				Value: []cmkapi.CryptoCertificate{{
					RootCA:  "CRoot",
					Subject: "CSub"}}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := clientcertificates.ToAPI(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.NoError(t, err)
		})
	}
}
