package oauth2

import (
	"crypto/tls"
	"errors"
	"net/http"

	"golang.org/x/oauth2/clientcredentials"
)

var (
	ErrClientIDMustBeSet = errors.New("clientID must be set")
	ErrTokenURLRequired  = errors.New("tokenURL must be set")
)

type ClientConfig struct {
	tlsConfig    *tls.Config
	apiClient    *http.Client
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	scopes       []string
}

type Option func(*ClientConfig)

func WithTLSConfig(cfg *tls.Config) Option {
	return func(c *ClientConfig) {
		c.tlsConfig = cfg
	}
}

func WithAPIClient(client *http.Client) Option {
	return func(c *ClientConfig) {
		c.apiClient = client
	}
}

func WithClientID(id string) Option {
	return func(c *ClientConfig) {
		c.clientID = id
	}
}

func WithClientSecret(secret string) Option {
	return func(c *ClientConfig) {
		c.clientSecret = secret
	}
}

func WithAuthURL(url string) Option {
	return func(c *ClientConfig) {
		c.authURL = url
	}
}

func WithTokenURL(url string) Option {
	return func(c *ClientConfig) {
		c.tokenURL = url
	}
}

func WithScopes(scopes ...string) Option {
	return func(c *ClientConfig) {
		c.scopes = append(c.scopes, scopes...)
	}
}

func NewClient(opts ...Option) (*http.Client, error) {
	cfg := &ClientConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	err := validate(cfg)
	if err != nil {
		return nil, err
	}

	tokenClient := http.DefaultClient
	if cfg.tlsConfig != nil {
		tokenClient.Transport = &http.Transport{
			TLSClientConfig: cfg.tlsConfig,
		}
	}

	oauth2Config := &clientcredentials.Config{
		ClientID:     cfg.clientID,
		ClientSecret: cfg.clientSecret,
		TokenURL:     cfg.tokenURL,
		Scopes:       cfg.scopes,
	}

	return &http.Client{
		Transport: NewTokenInjector(cfg.apiClient, tokenClient, oauth2Config),
	}, nil
}

func validate(b *ClientConfig) error {
	if b.clientID == "" && b.tokenURL == "" {
		return errors.Join(ErrClientIDMustBeSet, ErrTokenURLRequired)
	}

	if b.clientID == "" {
		return ErrClientIDMustBeSet
	}

	if b.tokenURL == "" {
		return ErrTokenURLRequired
	}

	return nil
}
