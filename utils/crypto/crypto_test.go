package crypto_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	cryptoUtils "github.com/openkcm/cmk/utils/crypto"
)

var (
	caHint         = base64.StdEncoding.EncodeToString([]byte("CACert"))
	clientCertHint = base64.StdEncoding.EncodeToString([]byte("clientCert"))

	ErrInvalidCert = errors.New("invalid certificate")
)

func TestLoadRSAPrivateKey_Pkcs1AndPkcs8(t *testing.T) {
	// Do not use <3k RSA keys in non-test-code!
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pkcs1Key := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs1Bytes,
	})

	pkcs8Bytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	pkcs8Key := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	loadedPkeyPkcs1, err := cryptoUtils.LoadRSAPrivateKey(pkcs1Key)
	assert.NoError(t, err)
	assert.NotNil(t, loadedPkeyPkcs1)

	loadedPkeyPkcs8, err := cryptoUtils.LoadRSAPrivateKey(pkcs8Key)
	assert.NoError(t, err)
	assert.NotNil(t, loadedPkeyPkcs8)

	assert.Equal(t, loadedPkeyPkcs1, loadedPkeyPkcs8)
}

func TestLoadRSAPrivateKey_Invalid(t *testing.T) {
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecPkcs8Bytes, _ := x509.MarshalPKCS8PrivateKey(ecKey)
	ecPkcs8Key := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: ecPkcs8Bytes,
	})

	tests := []struct {
		name string
		key  []byte
	}{
		{
			name: "LoadRSAPrivateKeyInvalid_Pkcs8",
			key:  []byte("-----BEGIN PRIVATE KEY-----\nNOTAVALIDKEY\n-----END PRIVATE KEY-----"),
		},
		{
			name: "LoadRSAPrivateKeyInvalid_Pkcs1",
			key: []byte(
				"-----BEGIN RSA PRIVATE KEY-----\nNOTAVALIDKEY\n-----END RSA PRIVATE KEY-----",
			),
		},
		{
			name: "LoadRSAPrivateKeyECkey",
			key:  ecPkcs8Key,
		},
		{
			name: "LoadRSAPrivateKeyInvalid_Type",
			key:  []byte("-----BEGIN INVALID KEY-----\nNOTAVALIDKEY\n-----END INVALID KEY-----"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadedKey, err := cryptoUtils.LoadRSAPrivateKey(tt.key)
			assert.Error(t, err)
			assert.Nil(t, loadedKey)
		})
	}
}

func TestSignWithRSAPrivateKey(t *testing.T) {
	// Do not use <3k RSA keys in non-test-code!
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	content := []byte("content")

	signature, err := cryptoUtils.SignWithRSAPrivateKey(privateKey, content)
	assert.NoError(t, err)

	signatureBytes, err := hex.DecodeString(signature)
	assert.NoError(t, err)

	hasher := sha256.New()
	hasher.Write(content)

	err = rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, hasher.Sum(nil), signatureBytes)
	assert.NoError(t, err)
}

func TestLoadCertificates(t *testing.T) {
	patchedParseCertificate := func(data []byte) (*x509.Certificate, error) {
		switch {
		case strings.Contains(string(data), "CACert"):
			return &x509.Certificate{IsCA: true}, nil
		case strings.Contains(string(data), "clientCert"):
			return &x509.Certificate{IsCA: false}, nil
		default:
			return nil, ErrInvalidCert
		}
	}
	cryptoUtils.PatchParseX509Certificate(patchedParseCertificate)

	tests := []struct {
		name        string
		cert        []byte
		expectError bool
		caChainLen  int
	}{
		{
			name: "TestLoadCertificates_NoCert",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				"notACert\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: true,
		},
		{
			name: "TestLoadCertificates_ClientOnly",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: false,
			caChainLen:  0,
		},
		{
			name: "TestLoadCertificates_CAOnly",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: true,
		},
		{
			name: "TestLoadCertificates_MultipleClientCerts",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: true,
		},
		{
			name: "TestLoadCertificates_PkeyInChain",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN PRIVATE KEY-----\n" +
				clientCertHint + "\n" +
				"-----END PRIVATE KEY-----\n"),
			expectError: true,
		},
		{
			name: "TestLoadCertificates_OneClientCertOneCA",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: false,
			caChainLen:  1,
		},
		{
			name: "TestLoadCertificates_OneClientCertMultipleCA",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: false,
			caChainLen:  2,
		},
		{
			name: "TestLoadCertificates_OneClientCertMultipleCA_ChangedOrder",
			cert: []byte("-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				clientCertHint + "\n" +
				"-----END CERTIFICATE-----\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				caHint + "\n" +
				"-----END CERTIFICATE-----\n"),
			expectError: false,
			caChainLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadedClientCert, loadedCAs, err := cryptoUtils.LoadCertificates(tt.cert)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, loadedClientCert)
				assert.Nil(t, loadedCAs)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, loadedClientCert)
				assert.Len(t, loadedCAs, tt.caChainLen)
			}
		})
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	var tests = []struct {
		name        string
		bitSize     int
		expectedErr bool
	}{
		{
			name:        "GeneratePrivateKey_SUCCESS",
			bitSize:     2048,
			expectedErr: false,
		},
		{
			name:        "GeneratePrivateKey_ERROR",
			bitSize:     1,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey, err := cryptoUtils.GeneratePrivateKey(tt.bitSize)
			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, privateKey)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, privateKey)
			}
		})
	}
}
