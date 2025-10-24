package hyokkey_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform/key/hyokkey"
	"github.com/openkcm/cmk-core/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

var (
	ErrAccessDetailsRequired    = errors.New("accessDetails is required")
	ErrManagementCryptoRequired = errors.New("management and crypto access details are required")
	ErrManagementCryptoEmpty    = errors.New("management and crypto access details cannot be empty")
)

type MockHYOKProviderTransformer struct{}

func (f MockHYOKProviderTransformer) ValidateAPI(_ context.Context, _ cmkapi.Key) error {
	panic("not implemented")
}

func (f MockHYOKProviderTransformer) SerializeKeyAccessData(
	_ context.Context, key cmkapi.Key,
) (*transformer.SerializedKeyAccessData, error) {
	if key.AccessDetails == nil {
		return nil, ErrAccessDetailsRequired
	}

	if key.AccessDetails.Management == nil || key.AccessDetails.Crypto == nil {
		return nil, ErrManagementCryptoRequired
	}

	if len(*key.AccessDetails.Management) == 0 || len(*key.AccessDetails.Crypto) == 0 {
		return nil, ErrManagementCryptoEmpty
	}

	managementData, err := json.Marshal(*key.AccessDetails.Management)
	if err != nil {
		return nil, err
	}

	cryptoData, err := json.Marshal(*key.AccessDetails.Crypto)
	if err != nil {
		return nil, err
	}

	return &transformer.SerializedKeyAccessData{
		Management: managementData,
		Crypto:     cryptoData,
	}, nil
}

func (f MockHYOKProviderTransformer) GetRegion(_ context.Context, _ cmkapi.Key) (string, error) {
	return "test-region", nil
}

func TestFromCmkAPIKey(t *testing.T) {
	tf := MockHYOKProviderTransformer{}
	tests := []struct {
		name     string
		apiKey   cmkapi.Key
		expected *model.Key
		errMsg   string
	}{
		{
			name: "Missing NativeID",
			apiKey: cmkapi.Key{
				Provider: ptr.PointTo("test-provider"),
			},
			expected: nil,
			errMsg:   "nativeID is required",
		},
		{
			name: "Missing AccessDetails",
			apiKey: cmkapi.Key{
				NativeID: ptr.PointTo("native-id"),
				Provider: ptr.PointTo("test-provider"),
			},
			expected: nil,
			errMsg:   "accessDetails is required",
		},
		{
			name: "Unexpected Algorithm",
			apiKey: cmkapi.Key{
				NativeID:  ptr.PointTo("native-id"),
				Provider:  ptr.PointTo("test-provider"),
				Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
			},
			expected: nil,
			errMsg:   "unexpected field: algorithm",
		},
		{
			name: "Unexpected Region",
			apiKey: cmkapi.Key{
				NativeID: ptr.PointTo("native-id"),
				Provider: ptr.PointTo("test-provider"),
				Region:   ptr.PointTo("us-east-1"),
			},
			expected: nil,
			errMsg:   "unexpected field: region",
		},
		{
			name: "Invalid AccessDetails",
			apiKey: cmkapi.Key{
				NativeID: ptr.PointTo("native-id"),
				Provider: ptr.PointTo("test-provider"),
				AccessDetails: &cmkapi.KeyAccessDetails{
					Management: ptr.PointTo(map[string]any{}),
				},
			},
			expected: nil,
			errMsg:   "error transforming access data from API to model: management and crypto access details are required",
		},
		{
			name: "Successful Transformation",
			apiKey: cmkapi.Key{
				NativeID: ptr.PointTo("native-id"),
				Provider: ptr.PointTo("TEST"),
				AccessDetails: &cmkapi.KeyAccessDetails{
					Management: ptr.PointTo(map[string]any{"key": "value"}),
					Crypto:     ptr.PointTo(map[string]any{"cryptoKey": "cryptoValue"}),
				},
			},
			expected: &model.Key{
				NativeID:             ptr.PointTo("native-id"),
				Provider:             "TEST",
				Region:               "test-region",
				ManagementAccessData: []byte(`{"key":"value"}`),
				CryptoAccessData:     []byte(`{"cryptoKey":"cryptoValue"}`),
			},
			errMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hyokkey.FromCmkAPIKey(t.Context(), tt.apiKey, tf)
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
