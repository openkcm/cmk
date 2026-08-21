package transformer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

func getPluginProviderTransformer(t *testing.T) *transformer.PluginProviderTransformer {
	t.Helper()

	svcRegistry := testutils.NewTestPlugins()

	tf, err := transformer.NewPluginProviderTransformer(svcRegistry, "TEST")
	assert.NoError(t, err)

	return tf
}

func TestValidatesAPIKey(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		Type:      cmkapi.KeyTypeBYOK,
		Algorithm: new(cmkapi.KeyAlgorithmAES256),
		Region:    new("us-east-1"),
		NativeID:  new("native-key-id"),
	}

	err := tf.ValidateAPI(t.Context(), key)

	assert.NoError(t, err)
}

func TestValidatesAPIKey_InvalidRegion(t *testing.T) {
	km := testplugins.NewTestKeyManagement(true, true).WithValidRegions("us-east-1")

	svcRegistry := testutils.NewTestPlugins(testplugins.WithKeyManagement("TEST", km))
	tf, err := transformer.NewPluginProviderTransformer(svcRegistry, "TEST")
	assert.NoError(t, err)

	key := cmkapi.Key{
		Type:      cmkapi.KeyTypeBYOK,
		Algorithm: new(cmkapi.KeyAlgorithmAES256),
		Region:    new("eu-west-1"),
	}

	gotErr := tf.ValidateAPI(t.Context(), key)

	assert.ErrorIs(t, gotErr, transformer.ErrValidateKey)
	assert.ErrorIs(t, gotErr, transformer.ErrGRPCValidateKey)

	var grpcErr errs.GRPCError
	assert.ErrorAs(t, gotErr, &grpcErr)
	assert.Equal(t, "region eu-west-1 is not supported", grpcErr.Reason)
}

func TestSerializesKeyAccessData(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		AccessDetails: &cmkapi.KeyAccessDetails{
			Management: &cmkapi.KeyAccessDetailsRegion{
				AdditionalProperties: map[string]any{
					"AccountID": "123456789012",
					"UserID":    "123456789012:user/test-user",
				},
			},
			Crypto: &map[string]cmkapi.KeyAccessDetailsRegion{
				"serviceA": {
					AdditionalProperties: map[string]any{
						"AccountID": "12344",
						"UserID":    "123456789012:user/serviceA",
					},
				},
				"serviceB": {
					AdditionalProperties: map[string]any{
						"AccountID": "12345",
						"UserID":    "123456789012:user/serviceB",
					},
				},
			},
		},
	}

	data, err := tf.SerializeKeyAccessData(t.Context(), key.AccessDetails)

	assert.NoError(t, err)
	assert.NotNil(t, data.Management)
	assert.NotNil(t, data.Crypto)
}

func TestSerializesKeyAccessData_Invalid(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		AccessDetails: &cmkapi.KeyAccessDetails{
			Management: &cmkapi.KeyAccessDetailsRegion{},
			Crypto:     new(map[string]cmkapi.KeyAccessDetailsRegion{}),
		},
	}

	_, err := tf.SerializeKeyAccessData(t.Context(), key.AccessDetails)
	assert.ErrorIs(t, err, transformer.ErrSerializeKeyAccessData)
}

func TestExtractRegion(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		NativeID: new("native-key-id"),
		AccessDetails: &cmkapi.KeyAccessDetails{
			Management: &cmkapi.KeyAccessDetailsRegion{
				AdditionalProperties: map[string]any{
					"key": "value",
				},
			},
		},
	}

	region, err := tf.GetRegion(t.Context(), key)

	assert.NoError(t, err)
	assert.Equal(t, "test-region", region)
}

func TestExtractRegion_InvalidNativeID(t *testing.T) {
	km := testplugins.NewTestKeyManagement(true, true).WithValidNativeIDPattern(`^valid-key/`)

	svcRegistry := testutils.NewTestPlugins(testplugins.WithKeyManagement("TEST", km))
	tf, err := transformer.NewPluginProviderTransformer(svcRegistry, "TEST")
	assert.NoError(t, err)

	key := cmkapi.Key{
		NativeID: new("invalid-native-id"),
	}

	_, gotErr := tf.GetRegion(t.Context(), key)

	assert.ErrorIs(t, gotErr, transformer.ErrExtractKeyRegion)
	assert.ErrorIs(t, gotErr, transformer.ErrGRPCValidateKey)

	var grpcErr errs.GRPCError
	assert.ErrorAs(t, gotErr, &grpcErr)
	assert.Contains(t, grpcErr.Reason, "invalid-native-id")
}
