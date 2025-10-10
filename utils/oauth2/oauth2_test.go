package oauth2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/utils/cert"
	"github.com/openkcm/cmk/utils/oauth2"
	"github.com/openkcm/cmk/utils/tlsconfig"
)

func TestNewClient(t *testing.T) {
	certPath, keyPath, err := cert.GenerateTemporaryCertAndKey()
	require.NoError(t, err)

	config, err := tlsconfig.NewTLSConfig(tlsconfig.WithCertAndKey(certPath, keyPath))
	require.NoError(t, err)

	tests := []struct {
		name          string
		opts          []oauth2.Option
		expectedError []error
	}{
		{
			name: "WithAuthURL",
			opts: []oauth2.Option{
				oauth2.WithClientID("test-client-id"),
				oauth2.WithClientSecret("test-client-secret"),
				oauth2.WithTokenURL("https://example.com/token"),
				oauth2.WithAuthURL("https://example.com/auth"),
				oauth2.WithScopes("scope1", "scope2"),
			},
		},
		{
			name: "NoTokenURL",
			opts: []oauth2.Option{
				oauth2.WithClientID("test-client-id"),
				oauth2.WithClientSecret("test-client-secret"),
			},
			expectedError: []error{oauth2.ErrTokenURLRequired},
		},
		{
			name: "NoClientID",
			opts: []oauth2.Option{
				oauth2.WithClientSecret("test-client-secret"),
				oauth2.WithTokenURL("https://example.com/token"),
			},
			expectedError: []error{oauth2.ErrClientIDMustBeSet},
		},
		{
			name: "NoClientIDNoTokenURL",
			opts: []oauth2.Option{
				oauth2.WithClientSecret("test-client-secret"),
			},
			expectedError: []error{
				oauth2.ErrClientIDMustBeSet,
				oauth2.ErrTokenURLRequired,
			},
		},
		{
			name: "SuccessCase",
			opts: []oauth2.Option{
				oauth2.WithClientID("test-client-id"),
				oauth2.WithClientSecret("test-client-secret"),
				oauth2.WithTokenURL("https://example.com/token"),
				oauth2.WithScopes("scope1", "scope2"),
			},
		},
		{
			name: "SuccessCaseWithTLS",
			opts: []oauth2.Option{
				oauth2.WithClientID("test-client-id"),
				oauth2.WithClientSecret("test-client-secret"),
				oauth2.WithTokenURL("https://example.com/token"),
				oauth2.WithScopes("scope1", "scope2"),
				oauth2.WithTLSConfig(config),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := oauth2.NewClient(tt.opts...)

			if tt.expectedError != nil {
				for _, expectedError := range tt.expectedError {
					assert.ErrorIs(t, err, expectedError)
				}

				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				if transport, ok := client.Transport.(*oauth2.TokenInjector); ok {
					assert.NotNil(t, transport)
				} else {
					t.Fatal("unexpected transport type")
				}
			}
		})
	}
}
