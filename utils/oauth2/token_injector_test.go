package oauth2_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	oauth2injector "github.tools.sap/kms/cmk/utils/oauth2"
)

var ErrForced = errors.New("forced error")

var expectedToken = "expected-token"

type mockTransport struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestTokenInjector_RoundTrip(t *testing.T) {
	apiClientOK := &http.Client{
		Transport: &mockTransport{
			RoundTripFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
		},
	}
	apiClientErr := &http.Client{
		Transport: &mockTransport{
			RoundTripFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, ErrForced
			},
		},
	}
	tokenClientOK := &http.Client{
		Transport: &mockTransport{
			RoundTripFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(
						strings.NewReader(
							`{"access_token":"` + expectedToken + `","token_type":"bearer"}`,
						),
					),
				}, nil
			},
		},
	}
	tokenClientErr := &http.Client{
		Transport: &mockTransport{
			RoundTripFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, ErrForced
			},
		},
	}
	tests := []struct {
		name          string
		apiClient     *http.Client
		tokenClient   *http.Client
		req           *http.Request
		expectedError error
		expectedToken string
		currentToken  *oauth2.Token
	}{
		{
			name:        "InjectsAuthorizationHeader",
			apiClient:   apiClientOK,
			tokenClient: tokenClientOK,
			req: func() *http.Request {
				req, _ := http.NewRequestWithContext(
					t.Context(), http.MethodGet, "https://example.com/resource", nil)

				return req
			}(),
			expectedError: nil,
			expectedToken: "Bearer " + expectedToken,
		},
		{
			name:        "InjectsAuthorizationHeaderTokenReuse",
			apiClient:   apiClientOK,
			tokenClient: tokenClientOK,
			req: func() *http.Request {
				req, _ := http.NewRequestWithContext(
					t.Context(), http.MethodGet, "https://example.com/resource", nil)

				return req
			}(),
			expectedError: nil,
			currentToken: &oauth2.Token{
				AccessToken: "current-token",
			},
			expectedToken: "Bearer " + "current-token",
		},
		{
			name:        "TokenRequestFailed",
			apiClient:   apiClientOK,
			tokenClient: tokenClientErr,
			req: func() *http.Request {
				req, _ := http.NewRequestWithContext(
					t.Context(), http.MethodGet, "https://example.com/resource", nil)

				return req
			}(),
			expectedError: oauth2injector.ErrFirstTokenRequestFailed,
		},
		{
			name:        "TokenRequestFailedTokenReuse",
			apiClient:   apiClientOK,
			tokenClient: tokenClientErr,
			req: func() *http.Request {
				req, _ := http.NewRequestWithContext(
					t.Context(), http.MethodGet, "https://example.com/resource", nil)

				return req
			}(),
			currentToken: &oauth2.Token{
				AccessToken: "current-token",
				Expiry:      time.Now().Add(-1),
			},
			expectedError: oauth2injector.ErrTokenReuse,
		},
		{
			name:        "APIFailed",
			apiClient:   apiClientErr,
			tokenClient: tokenClientOK,
			req: func() *http.Request {
				req, _ := http.NewRequestWithContext(
					t.Context(), http.MethodGet, "https://example.com/resource", nil)

				return req
			}(),
			expectedError: oauth2injector.ErrFailedToExecuteRequest,
		},
		{
			name:        "APIFailedTokenReuse",
			apiClient:   apiClientErr,
			tokenClient: tokenClientOK,
			req: func() *http.Request {
				req, _ := http.NewRequestWithContext(
					t.Context(), http.MethodGet, "https://example.com/resource", nil)

				return req
			}(),
			currentToken: &oauth2.Token{
				AccessToken: "current-token",
			},
			expectedError: oauth2injector.ErrFailedToExecuteRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := oauth2injector.NewTokenInjector(
				tt.apiClient,
				tt.tokenClient,
				&clientcredentials.Config{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					TokenURL:     "https://example.com/token",
				},
			)

			if tt.currentToken != nil {
				injector.SetToken(tt.currentToken)
			}

			resp, err := injector.RoundTrip(tt.req)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.expectedToken, tt.req.Header.Get("Authorization"))
			}

			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
		})
	}
}
