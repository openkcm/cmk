package testutils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/fullsailor/pkcs7"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/utils/crypto"
)

func CreateCertificateChain(
	t *testing.T,
	subject pkix.Name,
	pkey *rsa.PrivateKey,
) string {
	t.Helper()

	csr := &x509.CertificateRequest{
		Subject: subject,
	}

	var certChain []byte //nolint:prealloc

	certChain = append(certChain, CreateCertificatePEM(t, csr, pkey)...)
	certChain = append(certChain, CreateCACertificatePEM(t)...)

	var data []byte

	for len(certChain) > 0 {
		var block *pem.Block

		block, certChain = pem.Decode(certChain)
		data = append(data, block.Bytes...)
	}

	data, err := pkcs7.DegenerateCertificate(data)
	assert.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PKCS7",
		Bytes: data,
	}))
}

func CreateCertificatePEM(
	t *testing.T,
	csr *x509.CertificateRequest,
	pkey *rsa.PrivateKey,
) []byte {
	t.Helper()

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 1)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	assert.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               csr.Subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		IsCA:                  false,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, pkey.Public(), pkey)
	assert.NoError(t, err)

	cert, err := x509.ParseCertificate(certBytes)
	assert.NoError(t, err)

	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}

	return pem.EncodeToMemory(pemBlock)
}

func CreateCACertificatePEM(
	t *testing.T,
) []byte {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"test"},
			CommonName:   "test",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caPKey, _ := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		caPKey.Public(),
		caPKey)
	assert.NoError(t, err)

	caCert, err := x509.ParseCertificate(certBytes)
	assert.NoError(t, err)

	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCert.Raw,
	}

	return pem.EncodeToMemory(pemBlock)
}

type CryptoCert struct {
	Subject string
	RootCA  string
}
