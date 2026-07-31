package key_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/key"
	"github.com/openkcm/cmk/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	accessAccountIDField = "accessAccountID"
	accessUserIDField    = "accessUserID"
	regionEUWest1        = "eu-west-1"
)

type AccessDataTest struct {
	AccessAccountID string `json:"accessAccountID"`
	AccessUserID    string `json:"accessUserID"`
}

type ProviderTransformerTest struct{}

func (p ProviderTransformerTest) ValidateAPI(_ context.Context, _ cmkapi.Key) error {
	return nil
}

func (p ProviderTransformerTest) GetRegion(_ context.Context, _ cmkapi.Key) (string, error) {
	return regionEUWest1, nil
}

func (p ProviderTransformerTest) SerializeKeyAccessData(
	_ context.Context,
	accessDetails *cmkapi.KeyAccessDetails,
) (*transformer.SerializedKeyAccessData, error) {
	management, err := json.Marshal(accessDetails.Management)
	if err != nil {
		return nil, err
	}

	crypto, err := json.Marshal(accessDetails.Crypto)
	if err != nil {
		return nil, err
	}

	return &transformer.SerializedKeyAccessData{
		Management: management,
		Crypto:     crypto,
	}, nil
}

func (p ProviderTransformerTest) ValidateKeyAccessData(_ context.Context, _ *cmkapi.KeyAccessDetails) error {
	return nil
}

func TestTransformKeyFromAPI(t *testing.T) {
	description := "Test key"
	algorithm := cmkapi.KeyAlgorithmAES256
	keyType := cmkapi.KeyTypeBYOK
	nativeID := "native-id-1234"
	provider := "DUMMY"
	enabled := true
	disabled := false
	region := regionEUWest1
	keyConfigID := uuid.New()
	accessAccountID := "123456789012"
	accessUserID := "123456789012:user/test-user"

	// Define the mutator for model.Key SystemManagedKeyRequest
	ID := uuid.New()

	// Define the mutator for cmkapi.Key HYOKRequest
	HYOKKeyRequestMut := testutils.NewMutator(func() cmkapi.Key {
		return cmkapi.Key{
			Name:               "test-key",
			Type:               cmkapi.KeyTypeHYOK,
			KeyConfigurationID: keyConfigID,
			NativeID:           &nativeID,
			Enabled:            &enabled,
			Description:        &description,
			Provider:           &provider,
			AccessDetails: &cmkapi.KeyAccessDetails{
				Management: ptr.PointTo(map[string]any{
					accessAccountIDField: accessAccountID,
					accessUserIDField:    accessUserID,
				}),
			},
		}
	})

	// Define the mutator for model.Key SystemManagedKeyRequest
	modelHYOKKeyRequestMut := testutils.NewMutator(func() model.Key {
		return model.Key{
			ID:                 ID,
			Name:               "test-key",
			KeyType:            string(cmkapi.KeyTypeHYOK),
			KeyConfigurationID: keyConfigID,
			State:              cmkapi.KeyStateENABLED,
			Description:        description,
			NativeID:           &nativeID,
		}
	})

	invalidKeyRequestMut := testutils.NewMutator(func() cmkapi.Key {
		return cmkapi.Key{
			Name:               "test-key",
			Type:               keyType,
			KeyConfigurationID: keyConfigID,
			Algorithm:          &algorithm,
			Provider:           &provider,
			Region:             &region,
			Enabled:            &enabled,
			Description:        &description,
			NativeID:           &nativeID,
		}
	})
	tests := []struct {
		name     string
		apiKey   cmkapi.Key
		expected model.Key
		err      error
	}{
		{
			name:     "T203KeyFromAPIEnabledKeySuccess",
			apiKey:   HYOKKeyRequestMut(),
			expected: modelHYOKKeyRequestMut(),
			err:      nil,
		},
		{
			name: "T204KeyFromAPIEmptyEnabledSuccess",
			apiKey: HYOKKeyRequestMut(func(k *cmkapi.Key) {
				k.Enabled = nil
			}),
			expected: modelHYOKKeyRequestMut(func(_ *model.Key) {}),
			err:      nil,
		},
		{
			name: "T205KeyFromAPIDisabledSuccess",
			apiKey: HYOKKeyRequestMut(func(k *cmkapi.Key) {
				k.Enabled = &disabled
			}),
			expected: modelHYOKKeyRequestMut(func(k *model.Key) {
				k.State = cmkapi.KeyStateDISABLED
			}),
			err: nil,
		},
		{
			name: "T206KeyFromAPIMissingName",
			apiKey: HYOKKeyRequestMut(func(k *cmkapi.Key) {
				k.Name = ""
			}),
			expected: model.Key{},
			err:      apierrors.ErrNameFieldMissingProperty,
		},
		{
			name:     "T207KeyFromAPI invalid ApiKey with HOYK and System Managed Key Request values",
			apiKey:   invalidKeyRequestMut(),
			expected: model.Key{},
			err: errs.Wrap(
				keyshared.ErrFromAPI,
				errs.Wrapf(transform.ErrAPIUnexpectedProperty, "nativeID"),
			),
		},
		{
			name: "T208KeyFromAPIMissingType",
			apiKey: HYOKKeyRequestMut(func(k *cmkapi.Key) {
				k.Type = ""
			}),
			expected: model.Key{},
			err:      apierrors.ErrTypeFieldMissingProperty,
		},
		{
			name: "T209KeyFromAPIMissingKeyConfigurationID",
			apiKey: HYOKKeyRequestMut(func(k *cmkapi.Key) {
				k.KeyConfigurationID = uuid.Nil
			}),
			expected: model.Key{},
			err:      apierrors.ErrKeyConfigurationFieldMissingProperty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, err := key.FromAPI(t.Context(), tt.apiKey, ProviderTransformerTest{})
			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
				assert.Nil(t, k)
			} else {
				assert.NotEmpty(t, k.ID)
				assert.Equal(t, tt.expected.Name, k.Name)
				assert.Equal(t, tt.expected.State, k.State)
			}
		})
	}
}

func TestTransformKeyToAPI(t *testing.T) {
	id := uuid.New()
	keyConfigID := uuid.New()
	description := "Test key"

	tests := []struct {
		name     string
		key      model.Key
		expected cmkapi.Key
	}{
		{
			name: "Transform to HYOK Key",
			key: model.Key{
				ID:                 id,
				Name:               "test-key",
				KeyType:            string(cmkapi.KeyTypeHYOK),
				KeyConfigurationID: keyConfigID,
				Description:        description,
				NativeID:           ptr.PointTo("native-id-1234"),
				State:              cmkapi.KeyStateENABLED,
				IsPrimary:          false,
				ManagementAccessData: mustMarshal(AccessDataTest{
					AccessAccountID: "123456789012",
					AccessUserID:    "123456789012:user/test-user",
				}),
				CryptoAccessData: mustMarshal(map[string]AccessDataTest{
					"serviceA": {
						AccessAccountID: "12344",
						AccessUserID:    "123456789012:user/serviceA",
					},
					"serviceB": {
						AccessAccountID: "12345",
						AccessUserID:    "123456789012:user/serviceB",
					},
				}),
				EditableRegions: map[string]bool{
					"serviceA": true,
					"serviceB": false,
				},
			},
			expected: cmkapi.Key{
				Id:                 &id,
				Name:               "test-key",
				Type:               cmkapi.KeyTypeHYOK,
				KeyConfigurationID: keyConfigID,
				Description:        &description,
				NativeID:           ptr.PointTo("native-id-1234"),
				State:              ptr.PointTo(cmkapi.KeyStateENABLED),
				IsPrimary:          ptr.PointTo(false),
				AccessDetails: &cmkapi.KeyAccessDetails{
					Management: ptr.PointTo(map[string]any{
						accessAccountIDField: "123456789012",
						accessUserIDField:    "123456789012:user/test-user",
					}),
					Crypto: ptr.PointTo(map[string]map[string]any{
						"serviceA": {
							accessAccountIDField:           "12344",
							accessUserIDField:              "123456789012:user/serviceA",
							manager.IsEditableCryptoAccess: true,
						},
						"serviceB": {
							accessAccountIDField:           "12345",
							accessUserIDField:              "123456789012:user/serviceB",
							manager.IsEditableCryptoAccess: false,
						},
					}),
				},
				Metadata: &cmkapi.KeyMetadata{
					CreatedAt: &time.Time{},
					UpdatedAt: &time.Time{},
				},
				UnderWorkflow: ptr.PointTo(false),
			},
		},
		{
			name: "Transform to BYOK key",
			key: model.Key{
				ID:                 id,
				Name:               "byok-key",
				KeyType:            string(cmkapi.KeyTypeBYOK),
				KeyConfigurationID: keyConfigID,
				Description:        description,
				Algorithm:          string(cmkapi.KeyAlgorithmAES256),
				Region:             regionEUWest1,
				State:              cmkapi.KeyStateENABLED,
				IsPrimary:          true,
			},
			expected: cmkapi.Key{
				Id:                 &id,
				Name:               "byok-key",
				Type:               cmkapi.KeyTypeBYOK,
				KeyConfigurationID: keyConfigID,
				Description:        &description,
				Algorithm:          ptr.PointTo(cmkapi.KeyAlgorithmAES256),
				Region:             ptr.PointTo(regionEUWest1),
				State:              ptr.PointTo(cmkapi.KeyStateENABLED),
				IsPrimary:          ptr.PointTo(true),
				Metadata: &cmkapi.KeyMetadata{
					CreatedAt: &time.Time{},
					UpdatedAt: &time.Time{},
				},
				UnderWorkflow: ptr.PointTo(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey, err := key.ToAPI(tt.key)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, *apiKey)
		})
	}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
