package transformer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func getPluginProviderTransformer(t *testing.T) *transformer.PluginProviderTransformer {
	t.Helper()

	plugins := testutils.SetupMockPlugins(testutils.KeyStorePlugin)
	cfg := &config.Config{Plugins: plugins}
	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	tf, err := transformer.NewPluginProviderTransformer(ctlg, "TEST")
	assert.NoError(t, err)

	return tf
}

func TestValidatesAPIKey(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		Type:      cmkapi.KeyTypeBYOK,
		Algorithm: ptr.PointTo(cmkapi.KeyAlgorithmRSA3072),
		Region:    ptr.PointTo("us-east-1"),
		NativeID:  ptr.PointTo("native-key-id"),
	}

	err := tf.ValidateAPI(t.Context(), key)

	assert.NoError(t, err)
}

func TestSerializesKeyAccessData(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		AccessDetails: &cmkapi.KeyAccessDetails{
			Management: ptr.PointTo(map[string]any{
				"accountID": "123456789012",
				"userID":    "123456789012:user/test-user",
			}),
			Crypto: ptr.PointTo(map[string]map[string]any{
				"serviceA": {
					"accountID": "12344",
					"userID":    "123456789012:user/serviceA",
				},
				"serviceB": {
					"accountID": "12345",
					"userID":    "123456789012:user/serviceB",
				},
			}),
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
			Management: ptr.PointTo(map[string]any{}),
			Crypto:     ptr.PointTo(map[string]map[string]any{}),
		},
	}

	_, err := tf.SerializeKeyAccessData(t.Context(), key.AccessDetails)
	assert.ErrorIs(t, err, transformer.ErrSerializeKeyAccessData)
}

func TestExtractRegion(t *testing.T) {
	tf := getPluginProviderTransformer(t)

	key := cmkapi.Key{
		NativeID: ptr.PointTo("native-key-id"),
		AccessDetails: &cmkapi.KeyAccessDetails{
			Management: ptr.PointTo(map[string]any{"key": "value"}),
		},
	}

	region, err := tf.GetRegion(t.Context(), key)

	assert.NoError(t, err)
	assert.Equal(t, "test-region", region)
}
