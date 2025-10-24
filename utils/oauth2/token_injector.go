package oauth2

import (
	"context"
	"errors"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrFirstTokenRequestFailed = errors.New("failed to get token")
	ErrTokenReuse              = errors.New("failed to reuse token")
	ErrFailedToExecuteRequest  = errors.New("failed to execute request")
)

// TokenInjector is a struct that implements http.RoundTripper and injects headers into requests.
type TokenInjector struct {
	apiClient               *http.Client
	clientCredentialsConfig *clientcredentials.Config
	tokenClient             *http.Client
	token                   *oauth2.Token
}

// NewTokenInjector creates a new instance of TokenInjector.
// It requires an apiClient, tokenClient, and clientCredentialsConfig.
// The apiClient is used to execute HTTP requests.
// The tokenClient is used to fetch tokens.
// The clientCredentialsConfig is used to create a token source.
// The token is stored in the TokenInjector and reused for subsequent requests.
// If the token is nil or expired, a new token is fetched.
// In other cases, the token is reused.
// The token is injected into the request done by apiClient as a Bearer token.
func NewTokenInjector(
	apiClient *http.Client,
	tokenClient *http.Client,
	clientCredentialsConfig *clientcredentials.Config,
) *TokenInjector {
	return &TokenInjector{
		apiClient:               apiClient,
		tokenClient:             tokenClient,
		clientCredentialsConfig: clientCredentialsConfig,
	}
}

// RoundTrip executes a single HTTP transaction and injects headers into the request.
func (h *TokenInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	var token *oauth2.Token

	var err error

	if h.token == nil {
		token, err = h.login(req.Context())
		if err != nil {
			return nil, err
		}

		h.token = token
	} else {
		token, err = h.reuse(req.Context())
		if err != nil {
			return nil, err
		}
	}

	req.Header.Add("Authorization", "Bearer "+token.AccessToken)

	do, err := h.apiClient.Do(req)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToExecuteRequest, err)
	}

	return do, nil
}

// createTokenSource creates a new token source.
func (h *TokenInjector) createTokenSource(ctx context.Context) oauth2.TokenSource {
	tokenAPICtx := context.WithValue(ctx, oauth2.HTTPClient, h.tokenClient)
	return h.clientCredentialsConfig.TokenSource(tokenAPICtx)
}

// login is a helper function that logs in to the OAuth2 server.
func (h *TokenInjector) login(ctx context.Context) (*oauth2.Token, error) {
	token, err := h.createTokenSource(ctx).Token()
	if err != nil {
		return nil, errs.Wrap(ErrFirstTokenRequestFailed, err)
	}

	return token, nil
}

// reuse is a helper function that reuses the token.
func (h *TokenInjector) reuse(ctx context.Context) (*oauth2.Token, error) {
	source := h.createTokenSource(ctx)

	token, err := oauth2.ReuseTokenSource(h.token, source).Token()
	if err != nil {
		return nil, errs.Wrap(ErrTokenReuse, err)
	}

	return token, nil
}
