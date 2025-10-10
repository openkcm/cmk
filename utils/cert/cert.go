// Package cert provides utilities for generating X.509 certificates and private keys,
// including functionality for creating temporary certificate and key files.
//
// This package is primarily designed for **test purposes**, allowing developers
// to easily generate self-signed certificates and private keys for use in testing
// scenarios. It abstracts certificate creation and PEM encoding to facilitate mocking
// and testing of dependent components.
package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/openkcm/cmk/internal/errs"
)

// Error definitions for various failure scenarios in certificate and key generation.
var (
	ErrFailedToGeneratePrivateKey = errors.New("failed to generate private key")
	ErrFailedToCreateCertificate  = errors.New("failed to create certificate")
	ErrFailedToMarshalPrivateKey  = errors.New("failed to marshal private key")
	ErrFailedToWriteDataToCert    = errors.New("failed to write data to cert.pem")
	ErrFailedToWriteDataToKey     = errors.New("failed to write data to key.pem")
	ErrFailedToCreateCertTempFile = errors.New("failed to create temp file for Cert")
	ErrFailedToCreateKeyTempFile  = errors.New("failed to create temp file for key")
)

// PEMEncoder defines an interface for encoding data into PEM format.
// This abstraction allows for easier testing by enabling the mocking of PEM encoding.
//
// This interface is particularly useful in test scenarios where you want to
// validate how PEM encoding is handled without relying on the actual implementation.
type PEMEncoder interface {
	Encode(out io.Writer, block *pem.Block) error
}

// DefaultPEMEncoder is the default implementation of the PEMEncoder interface,
// using the standard library's pem.Encode function.
type DefaultPEMEncoder struct{}

// Encode writes a PEM-encoded block to the provided writer.
func (d *DefaultPEMEncoder) Encode(out io.Writer, block *pem.Block) error {
	return pem.Encode(out, block) //nolint:wrapcheck
}

// CertificateCreator defines an interface for creating X.509 certificates
// and marshaling ECDSA private keys. This abstraction facilitates testing by
// allowing custom implementations.
//
// By abstracting certificate creation, this interface enables developers to mock
// certificate generation logic in test cases.
type CertificateCreator interface {
	CreateCertificate(
		rand io.Reader,
		template, parent *x509.Certificate,
		pub, priv interface{},
	) ([]byte, error)
	MarshalECPrivateKey(key *ecdsa.PrivateKey) ([]byte, error)
}

// DefaultCertCreator is the default implementation of the CertificateCreator interface,
// using the standard library's x509.CreateCertificate and x509.MarshalECPrivateKey functions.
type DefaultCertCreator struct{}

// CreateCertificate generates an X.509 certificate based on the provided template,
// parent certificate, public key, and private key.
func (d *DefaultCertCreator) CreateCertificate(
	rand io.Reader,
	template, parent *x509.Certificate,
	pub, priv interface{},
) ([]byte, error) {
	return x509.CreateCertificate(rand, template, parent, pub, priv) //nolint:wrapcheck
}

// MarshalECPrivateKey marshals an ECDSA private key into DER format.
func (d *DefaultCertCreator) MarshalECPrivateKey(key *ecdsa.PrivateKey) ([]byte, error) {
	return x509.MarshalECPrivateKey(key) //nolint:wrapcheck
}

// GenerateTemporaryCertAndKey generates a self-signed X.509 certificate and corresponding private key,
// writing them to temporary files. It returns the paths to the generated certificate and key files.
//
// This function is primarily intended for **test purposes**, where we need to test,
// if client uses the certificate and key correctly.
//
// Returns:
//   - The path to the temporary certificate file.
//   - The path to the temporary private key file.
//   - An error if any part of the generation or writing process fails.
func GenerateTemporaryCertAndKey() (string, string, error) {
	return generateTempCertKeyPairWithCustomProviders(&DefaultCertCreator{}, &DefaultPEMEncoder{})
}

// generateTempCertKeyPairWithCustomProviders generates a self-signed X.509 certificate and private key,
// writing them to temporary files using custom implementations of CertificateCreator and PEMEncoder.
//
// This function is primarily intended for **test purposes**, where we need to test,
// if client uses the certificate and key correctly.
//
// Parameters:
//   - certCreator: A CertificateCreator implementation for creating certificates.
//   - pemEncoder: A PEMEncoder implementation for encoding data in PEM format.
//
// Returns:
//   - The path to the temporary certificate file.
//   - The path to the temporary private key file.
//   - An error if any part of the generation or writing process fails.
func generateTempCertKeyPairWithCustomProviders(
	certCreator CertificateCreator, pemEncoder PEMEncoder,
) (string, string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToGeneratePrivateKey, err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := certCreator.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&priv.PublicKey,
		priv,
	)
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToCreateCertificate, err)
	}

	certOut, err := os.CreateTemp("", fmt.Sprintf("cert-%d.pem", time.Now().Unix()))
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToCreateCertTempFile, err)
	}
	defer certOut.Close()

	err = pemEncoder.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToWriteDataToCert, err)
	}

	keyOut, err := os.CreateTemp("", fmt.Sprintf("key-%d.pem", time.Now().Unix()))
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToCreateKeyTempFile, err)
	}
	defer keyOut.Close()

	privBytes, err := certCreator.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToMarshalPrivateKey, err)
	}

	err = pemEncoder.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return "", "", errs.Wrap(ErrFailedToWriteDataToKey, err)
	}

	return certOut.Name(), keyOut.Name(), nil
}
