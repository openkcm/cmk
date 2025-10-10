package testutils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	rootCACertFile     = "rootCA.pem"
	rootCAKeyFile      = "rootCA.key"
	serverCertFile     = "server.pem"
	serverKeyFile      = "server.key"
	clientCertFile     = "client.pem"
	clientKeyFile      = "client.key"
	rabbitMQConfigFile = "rabbitmq.conf"

	rsaKeyBits = 2048

	serialCA     = 1
	serialServer = 2
	serialClient = 3

	filePermRO = 0o644
	filePermRW = 0o600
)

type TLSFiles struct {
	Dir            string
	RootCACertPath string
	RootCAKeyPath  string
	ServerCertPath string
	ServerKeyPath  string
	ClientCertPath string
	ClientKeyPath  string
}

func CreateTLSFiles(t *testing.T) TLSFiles {
	t.Helper()
	dir := t.TempDir()

	caKey, _ := rsa.GenerateKey(rand.Reader, rsaKeyBits) // #nosec G401 – test‑only key
	caTpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serialCA),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	assert.NoError(t, err, "certificate creation should not fail")
	writePEM(t, dir, rootCACertFile, "CERTIFICATE", caDER, filePermRO)
	writePEM(t, dir, rootCAKeyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey), filePermRW)

	caCert, err := x509.ParseCertificate(caDER)
	assert.NoError(t, err, "parsing CA certificate should not fail")

	srvKey, _ := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	srvTpl := &x509.Certificate{
		SerialNumber: big.NewInt(serialServer),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost", "rabbitmq", "solace"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}, //nolint:mnd
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTpl, caCert, &srvKey.PublicKey, caKey)
	assert.NoError(t, err, "certificate creation should not fail")
	writePEM(t, dir, serverCertFile, "CERTIFICATE", srvDER, filePermRO)
	writePEM(t, dir, serverKeyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(srvKey), filePermRW)

	cliKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	assert.NoError(t, err, "client key generation should not fail")

	cliTpl := &x509.Certificate{
		SerialNumber: big.NewInt(serialClient),
		Subject:      pkix.Name{CommonName: "guest"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	cliDER, err := x509.CreateCertificate(rand.Reader, cliTpl, caCert, &cliKey.PublicKey, caKey)
	assert.NoError(t, err, "client certificate creation should not fail")
	writePEM(t, dir, clientCertFile, "CERTIFICATE", cliDER, filePermRO)
	writePEM(t, dir, clientKeyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(cliKey), filePermRW)

	return TLSFiles{
		Dir:            dir,
		RootCACertPath: filepath.Join(dir, rootCACertFile),
		RootCAKeyPath:  filepath.Join(dir, rootCAKeyFile),
		ServerCertPath: filepath.Join(dir, serverCertFile),
		ServerKeyPath:  filepath.Join(dir, serverKeyFile),
		ClientCertPath: filepath.Join(dir, clientCertFile),
		ClientKeyPath:  filepath.Join(dir, clientKeyFile),
	}
}

func writePEM(t *testing.T, dir, name, typ string, der []byte, mode os.FileMode) {
	t.Helper()

	p := filepath.Join(dir, name)
	err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), mode)
	assert.NoError(t, err)
}
