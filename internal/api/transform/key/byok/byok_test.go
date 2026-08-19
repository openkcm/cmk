package byok_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/byok"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/model"
)

var (
	ErrProvider  = errors.New("provider must be 'TEST'")
	ErrRegion    = errors.New("region must be 'test-region'")
	ErrAlgorithm = errors.New("algorithm must be AES256")
)

type MockSysMrProviderTransformer struct{}

func (f MockSysMrProviderTransformer) ValidateAPI(_ context.Context, key cmkapi.Key) error {
	if *key.Provider != "TEST" {
		return ErrProvider
	}

	if *key.Region != "test-region" {
		return ErrRegion
	}

	if *key.Algorithm != cmkapi.KeyAlgorithmAES256 {
		return ErrAlgorithm
	}

	return nil
}

func (f MockSysMrProviderTransformer) SerializeKeyAccessData(_ context.Context, _ *cmkapi.KeyAccessDetails) (
	*transformer.SerializedKeyAccessData, error,
) {
	panic("not implemented")
}

func (f MockSysMrProviderTransformer) ValidateKeyAccessData(_ context.Context, _ *cmkapi.KeyAccessDetails) error {
	panic("not implemented")
}

func (f MockSysMrProviderTransformer) GetRegion(_ context.Context, _ cmkapi.Key) (string, error) {
	panic("not implemented")
}

func TestFromCmkAPIKey(t *testing.T) {
	tf := MockSysMrProviderTransformer{}
	tests := []struct {
		name     string
		apiKey   cmkapi.Key
		expected *model.Key
		errMsg   string
	}{
		{
			name: "Valid API Key",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeBYOK,
				Algorithm: new(cmkapi.KeyAlgorithmAES256),
				Region:    new("test-region"),
				Provider:  new("TEST"),
			},
			expected: &model.Key{
				KeyType:   cmkapi.KeyTypeBYOK,
				Algorithm: cmkapi.KeyAlgorithmAES256,
				Region:    "test-region",
				Provider:  "TEST",
			},
		},
		{
			name: "Missing Provider",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeBYOK,
				Algorithm: new(cmkapi.KeyAlgorithmAES256),
				Region:    new("test-region"),
			},
			errMsg: "provider is required",
		},
		{
			name: "Invalid Provider",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeBYOK,
				Algorithm: new(cmkapi.KeyAlgorithmAES256),
				Region:    new("test-region"),
				Provider:  new("INVALID"),
			},
			errMsg: "provider must be 'TEST'",
		},
		{
			name: "Missing Region",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeBYOK,
				Algorithm: new(cmkapi.KeyAlgorithmAES256),
				Provider:  new("TEST"),
			},
			errMsg: "region is required",
		},
		{
			name: "Invalid Region",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeBYOK,
				Algorithm: new(cmkapi.KeyAlgorithmAES256),
				Region:    new("invalid-region"),
				Provider:  new("TEST"),
			},
			errMsg: "region must be 'test-region'",
		},
		{
			name: "Missing Algorithm",
			apiKey: cmkapi.Key{
				Type:     cmkapi.KeyTypeBYOK,
				Region:   new("test-region"),
				Provider: new("TEST"),
			},
			errMsg: "algorithm is required",
		},
		{
			name: "Invalid Algorithm",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeBYOK,
				Algorithm: new(cmkapi.KeyAlgorithm("INVALID_ALG")),
				Region:    new("test-region"),
				Provider:  new("TEST"),
			},
			errMsg: "algorithm must be AES256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := byok.FromCmkAPIKey(t.Context(), tt.apiKey, tf)
			if tt.errMsg != "" {
				assert.Nil(t, result)
				assert.EqualError(t, err, tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
