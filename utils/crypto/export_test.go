package crypto

import (
	"crypto/x509"
)

type parseX509CertificateFuncType func(der []byte) (*x509.Certificate, error)

func PatchParseX509Certificate(patched parseX509CertificateFuncType) {
	parseX509Certificate = patched
}
