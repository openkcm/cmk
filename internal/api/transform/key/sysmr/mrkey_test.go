package sysmr_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/sysmr"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	ErrProvider  = errors.New("provider must be 'TEST'")
	ErrRegion    = errors.New("region must be 'test-region'")
	ErrAlgorithm = errors.New("algorithm must be RSA3072")
)

type MockSysMrProviderTransformer struct{}

func (f MockSysMrProviderTransformer) ValidateAPI(_ context.Context, key cmkapi.Key) error {
	if *key.Provider != "TEST" {
		return ErrProvider
	}

	if *key.Region != "test-region" {
		return ErrRegion
	}

	if *key.Algorithm != cmkapi.KeyAlgorithmRSA3072 {
		return ErrAlgorithm
	}

	return nil
}

func (f MockSysMrProviderTransformer) SerializeKeyAccessData(_ context.Context, _ cmkapi.Key) (
	*transformer.SerializedKeyAccessData, error) {
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
				Type:      cmkapi.KeyTypeSYSTEMMANAGED,
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
				Region:    ptr.PointTo("test-region"),
				Provider:  ptr.PointTo("TEST"),
			},
			expected: &model.Key{
				KeyType:   string(cmkapi.KeyTypeSYSTEMMANAGED),
				Algorithm: string(cmkapi.KeyAlgorithmRSA3072),
				Region:    "test-region",
				Provider:  "TEST",
			},
		},
		{
			name: "Missing Provider",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeSYSTEMMANAGED,
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
				Region:    ptr.PointTo("test-region"),
			},
			errMsg: "provider is required",
		},
		{
			name: "Invalid Provider",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeSYSTEMMANAGED,
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
				Region:    ptr.PointTo("test-region"),
				Provider:  ptr.PointTo("INVALID"),
			},
			errMsg: "provider must be 'TEST'",
		},
		{
			name: "Missing Region",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeSYSTEMMANAGED,
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
				Provider:  ptr.PointTo("TEST"),
			},
			errMsg: "region is required",
		},
		{
			name: "Invalid Region",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeSYSTEMMANAGED,
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
				Region:    ptr.PointTo("invalid-region"),
				Provider:  ptr.PointTo("TEST"),
			},
			errMsg: "region must be 'test-region'",
		},
		{
			name: "Missing Algorithm",
			apiKey: cmkapi.Key{
				Type:     cmkapi.KeyTypeSYSTEMMANAGED,
				Region:   ptr.PointTo("test-region"),
				Provider: ptr.PointTo("TEST"),
			},
			errMsg: "algorithm is required",
		},
		{
			name: "Invalid Algorithm",
			apiKey: cmkapi.Key{
				Type:      cmkapi.KeyTypeSYSTEMMANAGED,
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmAES256),
				Region:    ptr.PointTo("test-region"),
				Provider:  ptr.PointTo("TEST"),
			},
			errMsg: "algorithm must be RSA3072",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sysmr.FromCmkAPIKey(t.Context(), tt.apiKey, tf)
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
