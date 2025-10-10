package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

var (
	ErrCertificatesLoading  = errors.New("cert and key could not be loaded")
	ErrCaLoading            = errors.New("ca could not be loaded")
	ErrFailedToAppendCACert = errors.New("failed to append CA certificate to the pool")
)

type Option func(*tls.Config) error

func WithCertAndKey(certPath, keyPath string) Option {
	return func(c *tls.Config) error {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCertificatesLoading, err)
		}

		c.Certificates = []tls.Certificate{cert}

		return nil
	}
}

func WithCA(caPath string) Option {
	return func(c *tls.Config) error {
		caCert, err := os.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCaLoading, err)
		}

		if c.RootCAs == nil {
			c.RootCAs = x509.NewCertPool()
		}

		if !c.RootCAs.AppendCertsFromPEM(caCert) {
			return ErrFailedToAppendCACert
		}

		return nil
	}
}

func WithMinVersion(minVersion uint16) Option {
	return func(c *tls.Config) error {
		c.MinVersion = minVersion
		return nil
	}
}

func WithCertPool(pool *x509.CertPool) Option {
	return func(c *tls.Config) error {
		c.RootCAs = pool
		return nil
	}
}

func NewTLSConfig(opts ...Option) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	for _, opt := range opts {
		err := opt(config)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}
