package main_test

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"testing"

	"github.com/magodo/slog2hclog"
	"github.com/stretchr/testify/assert"

	keyopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	tp "github.com/openkcm/cmk-core/internal/testutils/testplugins/keystoreop"
	"github.com/openkcm/cmk-core/utils/ptr"
)

func setupTest() *tp.TestPlugin {
	p := tp.New()
	logLevelPlugin := new(slog.LevelVar)
	logLevelPlugin.Set(slog.LevelError)

	p.SetLogger(slog2hclog.New(slog.Default(), logLevelPlugin))

	return p
}

func TestGetKey(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	_, err := p.GetKey(t.Context(), &keyopv1.GetKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "mock-key/11111111"},
	})

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestGetKeyUpdateState(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	_, err := p.GetKey(t.Context(), &keyopv1.GetKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "mock-key/22222222"},
	})

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Act 2
	_, _ = p.DisableKey(t.Context(), &keyopv1.DisableKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "test-key-id"},
	})

	resp, err := p.GetKey(t.Context(), &keyopv1.GetKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "test-key-id"},
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.Equal(t, "DISABLED", resp.GetStatus(), "Expected key status to be DISABLED")
}

func TestCreateKeyVersion(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	resp, err := p.CreateKey(t.Context(), &keyopv1.CreateKeyRequest{
		Algorithm: keyopv1.KeyAlgorithm_KEY_ALGORITHM_AES256,
	})

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.NotEmpty(t, resp.GetKeyId())
}

func TestDeleteKeyVersion(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	_, err := p.DeleteKey(t.Context(), &keyopv1.DeleteKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "test-key-id"},
		Window:     ptr.PointTo(int32(7)),
	})

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestEnableKeyVersion(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	response, err := p.EnableKey(t.Context(), &keyopv1.EnableKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "test-key-id"},
	})

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.NotNil(t, response)
}

func TestEnableKeyVersion_Failed_EmptyRequest(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	response, err := p.EnableKey(t.Context(), nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestEnableKeyVersion_Failed_WrongParameter(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	response, err := p.EnableKey(t.Context(), &keyopv1.EnableKeyRequest{
		Parameters: nil,
	})

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)

	// Act
	response, err = p.EnableKey(t.Context(), &keyopv1.EnableKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: ""},
	})

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestDisableKeyVersion(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	response, err := p.DisableKey(t.Context(), &keyopv1.DisableKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "test-key-id"},
	})

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.NotNil(t, response)
}

func TestDisableKeyVersion_Failed_EmptyRequest(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	response, err := p.DisableKey(t.Context(), nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestDisableKeyVersion_Failed_WrongParameter(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	response, err := p.DisableKey(t.Context(), &keyopv1.DisableKeyRequest{
		Parameters: nil,
	})

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)

	// Act
	response, err = p.DisableKey(t.Context(), &keyopv1.DisableKeyRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: ""},
	})

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestGetImportParameters(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	resp, err := p.GetImportParameters(t.Context(), &keyopv1.GetImportParametersRequest{
		Parameters: &keyopv1.RequestParameters{KeyId: "test-key-id"},
		Algorithm:  keyopv1.KeyAlgorithm_KEY_ALGORITHM_AES256,
	})

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "CKM_RSA_AES_KEY_WRAP", resp.GetImportParameters().GetFields()["wrappingAlgorithm"].GetStringValue())
	assert.Equal(t, "SHA256", resp.GetImportParameters().GetFields()["hashFunction"].GetStringValue())
	assert.Equal(t, "mock-public-key-from-provider",
		resp.GetImportParameters().GetFields()["publicKey"].GetStringValue())
	assert.Equal(t, "mock-provider-params-from-provider",
		resp.GetImportParameters().GetFields()["providerParams"].GetStringValue())
}

func TestImportKeyMaterial(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	_, err := p.ImportKeyMaterial(t.Context(), &keyopv1.ImportKeyMaterialRequest{
		Parameters:           &keyopv1.RequestParameters{KeyId: "test-key-id"},
		EncryptedKeyMaterial: "abcdefghijklmnopqrstuvwxyz",
	})

	// Assert
	assert.NoError(t, err)
}

func TestTransformCryptoAccessData(t *testing.T) {
	p := setupTest()

	input := func() []byte {
		data := map[string]map[string]interface{}{
			"instance-1": {
				"field1": "value1",
				"field2": "value2",
			},
			"instance-2": {
				"field1": "value2",
				"field2": "value2",
			},
		}
		bytes, err := json.Marshal(data)
		assert.NoError(t, err)

		return bytes
	}()

	resp, err := p.TransformCryptoAccessData(t.Context(), &keyopv1.TransformCryptoAccessDataRequest{
		NativeKeyId: "test-key-id",
		AccessData:  input,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestConfigure(t *testing.T) {
	// Arrange
	p := setupTest()

	// Act
	res, err := p.Configure(t.Context(), nil)

	// Assert
	if err != nil {
		t.Errorf("Configure() error = %v, want nil", err)
	}

	if res == nil {
		t.Errorf("Configure() = nil, want non-nil")
	}
}

func TestNew(t *testing.T) {
	// Act
	p := tp.New()

	// Assert
	if p == nil {
		t.Errorf("Expected non-nil TestPlugin instance, got nil")
	}

	expectedType := "*main.TestPlugin"
	actualType := reflect.TypeOf(p).String()

	if actualType != expectedType {
		t.Errorf("Expected type %s, got %s", expectedType, actualType)
	}
}
