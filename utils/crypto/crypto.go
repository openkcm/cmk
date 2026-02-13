package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"

	"github.com/openkcm/cmk/internal/errs"
)

const (
	PEMArmorPKCS1RSAPrivateKey = "RSA PRIVATE KEY"
)

var (
	ErrGeneratePrivateKey = errors.New("generate private key error")
)

func GeneratePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// generate CA private key
	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, errs.Wrap(ErrGeneratePrivateKey, err)
	}

	return key, err
}
