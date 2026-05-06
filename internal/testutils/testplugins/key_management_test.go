package testplugins_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	"github.com/openkcm/cmk/utils/ptr"
)

func setupTest() *testplugins.TestKeyManagement {
	return testplugins.NewTestKeyManagement(true, true)
}

func TestGetKey(t *testing.T) {
	p := setupTest()

	_, err := p.GetKey(t.Context(), &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "mock-key/11111111"},
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestGetKeyUpdateState(t *testing.T) {
	p := setupTest()

	_, err := p.GetKey(t.Context(), &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "mock-key/22222222"},
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	_, _ = p.DisableKey(t.Context(), &keymanagement.DisableKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "test-key-id"},
	})

	resp, err := p.GetKey(t.Context(), &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "test-key-id"},
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.Equal(t, testplugins.DisabledKeyStatus, resp.Status, "Expected key status to be DISABLED")
}

func TestCreateKeyVersion(t *testing.T) {
	p := setupTest()

	resp, err := p.CreateKey(t.Context(), &keymanagement.CreateKeyRequest{
		KeyAlgorithm: keymanagement.AES256,
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.NotEmpty(t, resp.KeyID)
}

func TestDeleteKeyVersion(t *testing.T) {
	p := setupTest()

	_, err := p.DeleteKey(t.Context(), &keymanagement.DeleteKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "test-key-id"},
		Window:     ptr.PointTo(int32(7)),
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestEnableKeyVersion(t *testing.T) {
	p := setupTest()

	response, err := p.EnableKey(t.Context(), &keymanagement.EnableKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "test-key-id"},
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.NotNil(t, response)
}

func TestEnableKeyVersion_Failed_EmptyKeyID(t *testing.T) {
	p := setupTest()

	response, err := p.EnableKey(t.Context(), &keymanagement.EnableKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: ""},
	})

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestDisableKeyVersion(t *testing.T) {
	p := setupTest()

	response, err := p.DisableKey(t.Context(), &keymanagement.DisableKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: "test-key-id"},
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.NotNil(t, response)
}

func TestDisableKeyVersion_Failed_EmptyKeyID(t *testing.T) {
	p := setupTest()

	response, err := p.DisableKey(t.Context(), &keymanagement.DisableKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: ""},
	})

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestGetImportParameters(t *testing.T) {
	p := setupTest()

	resp, err := p.GetImportParameters(t.Context(), &keymanagement.GetImportParametersRequest{
		Parameters:   keymanagement.RequestParameters{KeyID: "test-key-id"},
		KeyAlgorithm: keymanagement.AES256,
	})

	assert.NoError(t, err)
	assert.Equal(t, "CKM_RSA_AES_KEY_WRAP", resp.ImportParameters["wrappingAlgorithm"])
	assert.Equal(t, "SHA256", resp.ImportParameters["hashFunction"])
	assert.Equal(t, "mock-public-key-from-provider", resp.ImportParameters["publicKey"])
	assert.Equal(t, "mock-provider-params-from-provider", resp.ImportParameters["providerParams"])
}

func TestImportKeyMaterial(t *testing.T) {
	p := setupTest()

	_, err := p.ImportKeyMaterial(t.Context(), &keymanagement.ImportKeyMaterialRequest{
		Parameters:           keymanagement.RequestParameters{KeyID: "test-key-id"},
		EncryptedKeyMaterial: "abcdefghijklmnopqrstuvwxyz",
	})

	assert.NoError(t, err)
}

func TestTransformCryptoAccessData(t *testing.T) {
	p := setupTest()

	input := func() []byte {
		data := map[string]map[string]any{
			"instance-1": {"field1": "value1", "field2": "value2"},
			"instance-2": {"field1": "value2", "field2": "value2"},
		}
		b, err := json.Marshal(data)
		assert.NoError(t, err)
		return b
	}()

	resp, err := p.TransformCryptoAccessData(t.Context(), &keymanagement.TransformCryptoAccessDataRequest{
		NativeKeyID: "test-key-id",
		AccessData:  input,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestConfigure(t *testing.T) {
	p := setupTest()

	cfg := p.ServiceInfo()
	assert.Equal(t, testplugins.Name, cfg.Name())
}

func TestDeleteKeyVersion_SetsStatus(t *testing.T) {
	p := setupTest()

	keyID := "mock-key/11111111"
	_, err := p.DeleteKey(t.Context(), &keymanagement.DeleteKeyRequest{
		Parameters: keymanagement.RequestParameters{KeyID: keyID},
	})
	assert.NoError(t, err)

	resp, err := p.GetKey(t.Context(), &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{
			KeyID:  keyID,
			Config: common.KeystoreConfig{Values: map[string]any{}},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, testplugins.PendingDeletionKeyStatus, resp.Status)
}
