package hyokkey_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/hyokkey"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
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

func (f MockHYOKProviderTransformer) ValidateKeyAccessData(
	_ context.Context, accessDetails *cmkapi.KeyAccessDetails,
) error {
	if accessDetails == nil {
		return ErrAccessDetailsRequired
	}

	if accessDetails.Management == nil || accessDetails.Crypto == nil {
		return ErrManagementCryptoRequired
	}

	if len(*accessDetails.Management) == 0 || len(*accessDetails.Crypto) == 0 {
		return ErrManagementCryptoEmpty
	}

	return nil
}

func (f MockHYOKProviderTransformer) SerializeKeyAccessData(
	ctx context.Context, accessDetails *cmkapi.KeyAccessDetails,
) (*transformer.SerializedKeyAccessData, error) {
	err := f.ValidateKeyAccessData(ctx, accessDetails)
	if err != nil {
		return nil, err
	}

	managementData, err := json.Marshal(accessDetails.Management)
	if err != nil {
		return nil, err
	}

	cryptoData, err := json.Marshal(accessDetails.Crypto)
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
					Crypto: ptr.PointTo(map[string]map[string]any{"cryptoRegion": {
						"cryptoKey": "cryptoValue",
					}}),
				},
			},
			expected: &model.Key{
				NativeID:             ptr.PointTo("native-id"),
				Provider:             "TEST",
				Region:               "test-region",
				ManagementAccessData: []byte(`{"key":"value"}`),
				CryptoAccessData:     []byte(`{"cryptoRegion":{"cryptoKey":"cryptoValue"}}`),
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
