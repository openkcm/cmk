package tlsconfig_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/utils/cert"
	"github.com/openkcm/cmk/utils/tlsconfig"
)

func TestCustomCertPool(t *testing.T) {
	customCertPool := x509.NewCertPool()
	require.NotNil(t, customCertPool)

	tlsConfig, err := tlsconfig.NewTLSConfig(
		tlsconfig.WithCertPool(customCertPool),
	)

	require.NoError(t, err)
	assert.Equal(t, customCertPool, tlsConfig.RootCAs)
}

func TestAppendCACertificate(t *testing.T) {
	caPath, _, err := cert.GenerateTemporaryCertAndKey()
	require.NoError(t, err)

	tlsConfig, err := tlsconfig.NewTLSConfig(
		tlsconfig.WithCA(caPath),
	)

	require.NoError(t, err)
	assert.NotNil(t, tlsConfig.RootCAs)
}

func TestInvalidCACertificate(t *testing.T) {
	caPath := "testdata/invalid_ca.pem"

	_, err := tlsconfig.NewTLSConfig(
		tlsconfig.WithCA(caPath),
	)

	require.ErrorIs(t, err, tlsconfig.ErrCaLoading)
}

func TestMinTLSVersion(t *testing.T) {
	tlsConfig, err := tlsconfig.NewTLSConfig(
		tlsconfig.WithMinVersion(tls.VersionTLS13),
	)

	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
}

func TestNoCertificatesProvided(t *testing.T) {
	tlsConfig, err := tlsconfig.NewTLSConfig()
	require.NoError(t, err)
	assert.Empty(t, tlsConfig.Certificates)
}

func TestInvalidCertificateAndKeyPair(t *testing.T) {
	certPath := "testdata/invalid_cert.pem"
	keyPath := "testdata/invalid_key.pem"

	_, err := tlsconfig.NewTLSConfig(
		tlsconfig.WithCertAndKey(certPath, keyPath),
	)

	require.ErrorIs(t, err, tlsconfig.ErrCertificatesLoading)
}

func TestPartialCertKeyArguments(t *testing.T) {
	t.Run("empty cert path", func(t *testing.T) {
		_, err := tlsconfig.NewTLSConfig(
			tlsconfig.WithCertAndKey("", "key.pem"),
		)
		require.ErrorContains(t, err, "cert and key could not be loaded")
	})

	t.Run("empty key path", func(t *testing.T) {
		_, err := tlsconfig.NewTLSConfig(
			tlsconfig.WithCertAndKey("cert.pem", ""),
		)
		require.ErrorContains(t, err, "cert and key could not be loaded")
	})
}

func TestValidCustomCertificateAndKeyPair(t *testing.T) {
	certPath, keyPath, err := cert.GenerateTemporaryCertAndKey()
	require.NoError(t, err)

	tlsConfig, err := tlsconfig.NewTLSConfig(
		tlsconfig.WithCertAndKey(certPath, keyPath),
	)

	require.NoError(t, err)
	assert.NotEmpty(t, tlsConfig.Certificates)
}
