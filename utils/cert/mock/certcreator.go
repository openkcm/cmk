package mock

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"io"
)

var (
	ErrForcedError             = errors.New("simulated certificate creation failure")
	ErrMarshalPrivateKeyFailed = errors.New("failed to marshal private key")
)

// CertCreator is a mock certificate creator
type CertCreator struct {
	ShouldReturnErrorForCert bool
	ShouldReturnErrorForKey  bool
}

// CreateCertificate creates a certificate
func (c *CertCreator) CreateCertificate(
	io.Reader,
	*x509.Certificate, *x509.Certificate,
	interface{}, interface{}) ([]byte, error) {
	if c.ShouldReturnErrorForCert {
		return nil, ErrForcedError
	}

	return []byte("mock-certificate"), nil
}

func (c *CertCreator) MarshalECPrivateKey(*ecdsa.PrivateKey) ([]byte, error) {
	if c.ShouldReturnErrorForKey {
		return nil, ErrMarshalPrivateKeyFailed
	}

	return []byte("mock-certificate"), nil
}
