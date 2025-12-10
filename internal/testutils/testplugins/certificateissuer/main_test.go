package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	certificate_issuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	tp "github.tools.sap/kms/cmk/internal/testutils/testplugins/certificateissuer"
)

func TestConfigureReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.Configure(t.Context(), &configv1.ConfigureRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetReturnsEmptyResponse(t *testing.T) {
	plugin := tp.New()
	resp, err := plugin.GetCertificate(t.Context(), &certificate_issuerv1.GetCertificateRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewCreatesTestPluginInstance(t *testing.T) {
	plugin := tp.New()
	assert.NotNil(t, plugin)
	assert.Implements(t, (*certificate_issuerv1.CertificateIssuerServiceServer)(nil), plugin)
}
