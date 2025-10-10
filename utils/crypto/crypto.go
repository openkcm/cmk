package crypto

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"

	"github.com/openkcm/cmk/internal/errs"
)

const (
	PEMArmorCertificate        = "CERTIFICATE"
	PEMArmorPKCS8PrivateKey    = "PRIVATE KEY"
	PEMArmorPKCS1RSAPrivateKey = "RSA PRIVATE KEY"
)

var (
	ErrNoClientCertificatesFound     = errors.New("no client certificates found")
	ErrMultipleClientCertificates    = errors.New("multiple client certificates found")
	ErrInvalidTypeInCertificateChain = errors.New(
		"a certificate in the chain is of the wrong type",
	)
	ErrFailedToParsePrivateKey  = errors.New("failed to parse private key")
	ErrFailedToParseCertificate = errors.New("failed to parse certificate")
	ErrPrivateKeyWrongType      = errors.New("private key is of the wrong type")
	ErrFailedToSignWithRSAKey   = errors.New("failed to sign using RSA private key")

	ErrPemEncode          = errors.New("PEM encode error")
	ErrGeneratePrivateKey = errors.New("generate private key error")
)

// calculateSHA256 calculates the SHA256 hash of the content.
func calculateSHA256(content []byte) []byte {
	hasher := sha256.New()
	hasher.Write(content)

	return hasher.Sum(nil)
}

// Sha256HashHex returns the SHA256 hash of the content as a hex string
func Sha256HashHex(content []byte) string {
	return hex.EncodeToString(calculateSHA256(content))
}

// SignWithRSAPrivateKey signs the content with the RSA private key and returns the signature as a hex string.
// The content is hashed with SHA256 before signing and uses PKCS#1 v1.5 padding.
func SignWithRSAPrivateKey(privateKey *rsa.PrivateKey, content []byte) (string, error) {
	hashedContent := calculateSHA256(content)

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashedContent)
	if err != nil {
		return "", errs.Wrap(ErrFailedToSignWithRSAKey, err)
	}

	return hex.EncodeToString(signature), nil
}

// LoadRSAPrivateKey loads the RSA private key from the given bytes
// either from PKCS1 or PKCS8 format.
func LoadRSAPrivateKey(privateKeyBytes []byte) (*rsa.PrivateKey, error) {
	var (
		parsedPrivateKey any
		err              error
	)

	pemLoaded, _ := pem.Decode(privateKeyBytes)
	switch pemLoaded.Type {
	case PEMArmorPKCS1RSAPrivateKey:
		parsedPrivateKey, err = x509.ParsePKCS1PrivateKey(pemLoaded.Bytes)
		if err != nil {
			return nil, errs.Wrap(ErrFailedToParsePrivateKey, err)
		}
	case PEMArmorPKCS8PrivateKey:
		parsedPrivateKey, err = x509.ParsePKCS8PrivateKey(pemLoaded.Bytes)
		if err != nil {
			return nil, errs.Wrap(ErrFailedToParsePrivateKey, err)
		}
	default:
		return nil, ErrPrivateKeyWrongType
	}

	privateKey, ok := parsedPrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrPrivateKeyWrongType
	}

	return privateKey, nil
}

// Assign the x509.ParseCertificate function to a variable to allow mocking in tests.
var parseX509Certificate = x509.ParseCertificate

// LoadCertificates loads the client certificate and certificate authorities from a PEM certificate chain.
// There must be one and only one client certificate in the chain.
// There can be zero, one, or more certificate authorities in the chain.
// Returns the singular client certificate and array of certificate authorities.
func LoadCertificates(certificateBytes []byte) (*x509.Certificate, []*x509.Certificate, error) {
	remainder := certificateBytes

	var (
		pemLoaded              *pem.Block
		clientCerts            []x509.Certificate
		certificateAuthorities []*x509.Certificate
	)

	// Iterate over the PEM blocks in the certificate chain and parse them
	// into x509.Certificate objects separating client certificates and certificate authorities.
	for {
		pemLoaded, remainder = pem.Decode(remainder)
		if pemLoaded == nil {
			break
		}

		if pemLoaded.Type != PEMArmorCertificate {
			return nil, nil, ErrInvalidTypeInCertificateChain
		}

		parsedCert, err := parseX509Certificate(pemLoaded.Bytes)
		if err != nil {
			return nil, nil, errs.Wrap(ErrFailedToParseCertificate, err)
		}

		if parsedCert.IsCA {
			certificateAuthorities = append(certificateAuthorities, parsedCert)
		} else {
			clientCerts = append(clientCerts, *parsedCert)
		}
	}

	switch len(clientCerts) {
	case 0:
		return nil, nil, ErrNoClientCertificatesFound
	case 1:
		return &clientCerts[0], certificateAuthorities, nil
	default:
		return nil, nil, ErrMultipleClientCertificates
	}
}

func PEMEncode(buffer []byte, pemType string) ([]byte, error) {
	buf := new(bytes.Buffer)

	err := pem.Encode(buf, &pem.Block{
		Type:  pemType,
		Bytes: buffer,
	})
	if err != nil {
		return nil, errs.Wrap(ErrPemEncode, err)
	}

	return buf.Bytes(), nil
}

func GeneratePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// generate CA private key
	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, errs.Wrap(ErrGeneratePrivateKey, err)
	}

	return key, err
}
