package key_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/transform"
	"github.tools.sap/kms/cmk/internal/api/transform/key"
	"github.tools.sap/kms/cmk/internal/api/transform/key/keyshared"
	"github.tools.sap/kms/cmk/internal/api/transform/key/transformer"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/testutils"
	"github.tools.sap/kms/cmk/utils/ptr"
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
	keyType := cmkapi.KeyTypeSYSTEMMANAGED
	nativeID := "native-id-1234"
	provider := "DUMMY"
	enabled := true
	disabled := false
	region := regionEUWest1
	keyConfigID := uuid.New()
	accessAccountID := "123456789012"
	accessUserID := "123456789012:user/test-user"

	// Define the mutator for cmkapi.Key SystemManagedKeyRequest
	ManagedKeyRequestMut := testutils.NewMutator(func() cmkapi.Key {
		return cmkapi.Key{
			Name:               "test-key",
			Type:               keyType,
			KeyConfigurationID: keyConfigID,
			Algorithm:          &algorithm,
			Provider:           &provider,
			Region:             &region,
			Enabled:            &enabled,
			Description:        &description,
		}
	})

	// Define the mutator for model.Key SystemManagedKeyRequest
	ID := uuid.New()
	modelManagedKeyRequestMut := testutils.NewMutator(func() model.Key {
		return model.Key{
			ID:                 ID,
			Name:               "test-key",
			KeyType:            string(keyType),
			KeyConfigurationID: keyConfigID,
			State:              string(cmkapi.KeyStateENABLED),
			Description:        description,
			Algorithm:          string(algorithm),
			Provider:           provider,
			Region:             region,
		}
	})

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
			State:              string(cmkapi.KeyStateENABLED),
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
			name:     "KeyFromAPI - Enabled Key Success",
			apiKey:   ManagedKeyRequestMut(),
			expected: modelManagedKeyRequestMut(),
			err:      nil,
		},
		{
			name: "KeyFromAPI - Empty Enabled Success",
			apiKey: ManagedKeyRequestMut(func(k *cmkapi.Key) {
				k.Enabled = nil
			}),
			expected: modelManagedKeyRequestMut(func(_ *model.Key) {}),
			err:      nil,
		},
		{
			name: "T201KeyFromAPIDisabledSuccess",
			apiKey: ManagedKeyRequestMut(func(k *cmkapi.Key) {
				k.Enabled = &disabled
			}),
			expected: modelManagedKeyRequestMut(func(k *model.Key) {
				k.State = string(cmkapi.KeyStateDISABLED)
			}),
			err: nil,
		},
		{
			name: "T202KeyFromAPIMissingName",
			apiKey: ManagedKeyRequestMut(func(k *cmkapi.Key) {
				k.Name = ""
			}),
			expected: model.Key{},
			err:      apierrors.ErrNameFieldMissingProperty,
		},
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
				k.State = string(cmkapi.KeyStateDISABLED)
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
	description := "Test key"
	algorithm := cmkapi.KeyAlgorithmAES256
	nativeID := "native-id-1234"
	provider := "TEST"
	region := regionEUWest1
	id := uuid.New()
	keyConfigID := uuid.New()

	keyVersion := model.KeyVersion{
		ExternalID: uuid.New().String(),
		KeyID:      id,
		AutoTimeModel: model.AutoTimeModel{
			CreatedAt: time.Now(),
		},
		Version:   1,
		IsPrimary: true,
	}

	modelManagedKeyRequestMut := testutils.NewMutator(func() model.Key {
		return model.Key{
			ID:                 id,
			Name:               "test-key",
			KeyType:            string(cmkapi.KeyTypeSYSTEMMANAGED),
			KeyConfigurationID: keyConfigID,
			Provider:           provider,
			Region:             region,
			Description:        description,
			State:              string(cmkapi.KeyStateENABLED),
			Algorithm:          "AES256",
			KeyVersions:        []model.KeyVersion{keyVersion},
		}
	})

	ManagedKeyRequestMut := testutils.NewMutator(func() cmkapi.Key {
		return cmkapi.Key{
			Id:                 AnyPtr(uuid.MustParse(id.String())),
			Name:               "test-key",
			Type:               cmkapi.KeyTypeSYSTEMMANAGED,
			KeyConfigurationID: keyConfigID,
			Description:        &description,
			Provider:           &provider,
			Region:             &region,
			Algorithm:          &algorithm,
			State:              AnyPtr(cmkapi.KeyStateENABLED),
			IsPrimary:          ptr.PointTo(false),
			Metadata: &cmkapi.KeyMetadata{
				CreatedAt:      AnyPtr(time.Time{}),
				UpdatedAt:      AnyPtr(time.Time{}),
				PrimaryVersion: AnyPtr(1),
				TotalVersions:  AnyPtr(1),
			},
		}
	})

	managementAccessData := AccessDataTest{
		AccessAccountID: "123456789012",
		AccessUserID:    "123456789012:user/test-user",
	}
	managementAccessJSON, err := json.Marshal(managementAccessData)
	assert.NoError(t, err)

	cryptoAccessData := map[string]AccessDataTest{
		"serviceA": {
			AccessAccountID: "12344",
			AccessUserID:    "123456789012:user/serviceA",
		},
		"serviceB": {
			AccessAccountID: "12345",
			AccessUserID:    "123456789012:user/serviceB",
		},
	}
	cryptoAccessJSON, err := json.Marshal(cryptoAccessData)
	assert.NoError(t, err)

	modelHYOKKeyRequestMut := testutils.NewMutator(func() model.Key {
		return model.Key{
			ID:                   id,
			Name:                 "test-key",
			KeyType:              string(cmkapi.KeyTypeHYOK),
			KeyConfigurationID:   keyConfigID,
			Description:          description,
			NativeID:             &nativeID,
			State:                string(cmkapi.KeyStateENABLED),
			IsPrimary:            false,
			ManagementAccessData: managementAccessJSON,
			CryptoAccessData:     cryptoAccessJSON,
		}
	})

	HYOKKeyRequestMut := testutils.NewMutator(func() cmkapi.Key {
		return cmkapi.Key{
			Id:                 AnyPtr(uuid.MustParse(id.String())),
			Name:               "test-key",
			Type:               cmkapi.KeyTypeHYOK,
			KeyConfigurationID: keyConfigID,
			Description:        &description,
			NativeID:           &nativeID,
			State:              AnyPtr(cmkapi.KeyStateENABLED),
			IsPrimary:          ptr.PointTo(false),
			AccessDetails: &cmkapi.KeyAccessDetails{
				Management: ptr.PointTo(map[string]any{
					accessAccountIDField: "123456789012",
					accessUserIDField:    "123456789012:user/test-user",
				}),
				Crypto: ptr.PointTo(map[string]any{
					"serviceA": map[string]any{
						accessAccountIDField: "12344",
						accessUserIDField:    "123456789012:user/serviceA",
					},
					"serviceB": map[string]any{
						accessAccountIDField: "12345",
						accessUserIDField:    "123456789012:user/serviceB",
					},
				}),
			},
			Metadata: &cmkapi.KeyMetadata{
				CreatedAt: AnyPtr(time.Time{}),
				UpdatedAt: AnyPtr(time.Time{}),
			},
		}
	})

	tests := []struct {
		name      string
		key       model.Key
		expected  cmkapi.Key
		expectErr bool
		err       error
	}{
		{
			name:      "T210KeyToAPISuccess",
			key:       modelManagedKeyRequestMut(),
			expected:  ManagedKeyRequestMut(),
			expectErr: false,
		},
		{
			name:      "T212KeyToAPISuccess",
			key:       modelHYOKKeyRequestMut(),
			expected:  HYOKKeyRequestMut(),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey, err := key.ToAPI(tt.key)
			if tt.expectErr {
				assert.EqualError(t, err, tt.err.Error())
				assert.Nil(t, apiKey)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, *apiKey)
			}
		})
	}
}

// AnyPtr returns a pointer to the given value of any type.
func AnyPtr[T any](v T) *T {
	return &v
}
