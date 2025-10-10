package manager_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"

	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestBuildImportParams(t *testing.T) {
	validTime := time.Now().Add(24 * time.Hour)
	validTimeStr := validTime.Format(time.RFC3339)
	publicKey := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0B..."
	wrappingAlgorithm := "RSAES_OAEP_SHA_256"
	hashFunction := "SHA_256"
	providerParams := "example-import-token"

	key := testutils.NewKey(func(k *model.Key) {
		k.Provider = providerTest
	})

	t.Run("AWS_ValidParams", func(t *testing.T) {
		fields := map[string]any{
			"publicKey":         publicKey,
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
			"providerParams":    providerParams,
			"validTo":           validTimeStr,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, key.ID, result.KeyID)
		assert.Equal(t, publicKey, result.PublicKeyPEM)
		assert.Equal(t, wrappingAlgorithm, result.WrappingAlg)
		assert.Equal(t, hashFunction, result.HashFunction)
		assert.NotNil(t, result.Expires)
		assert.WithinDuration(t, validTime, *result.Expires, time.Second)

		var providerParamsResult map[string]any

		err = json.Unmarshal(result.ProviderParameters, &providerParamsResult)
		assert.NoError(t, err)
		assert.Equal(t, providerParams, providerParamsResult["providerParams"])
	})

	t.Run("AWS_MissingPublicKey", func(t *testing.T) {
		fields := map[string]any{
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
			"providerParams":    providerParams,
			"validTo":           validTimeStr,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWS_MissingWrappingAlgorithm", func(t *testing.T) {
		fields := map[string]any{
			"publicKey":      publicKey,
			"hashFunction":   hashFunction,
			"providerParams": providerParams,
			"validTo":        validTimeStr,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWS_MissingHashFunction", func(t *testing.T) {
		fields := map[string]any{
			"publicKey":         publicKey,
			"wrappingAlgorithm": wrappingAlgorithm,
			"providerParams":    providerParams,
			"validTo":           validTimeStr,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWS_MissingImportToken", func(t *testing.T) {
		fields := map[string]any{
			"publicKey":         publicKey,
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
			"validTo":           validTimeStr,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWS_MissingValidTo", func(t *testing.T) {
		fields := map[string]any{
			"publicKey":         publicKey,
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
			"providerParams":    providerParams,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWS_InvalidValidToFormat", func(t *testing.T) {
		fields := map[string]any{
			"publicKey":         publicKey,
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
			"providerParams":    providerParams,
			"validTo":           "invalid-date-format",
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("UnsupportedProvider", func(t *testing.T) {
		keyUnknownProvider := testutils.NewKey(func(k *model.Key) {
			k.Provider = "UNKNOWN_PROVIDER"
		})
		fields := map[string]any{
			"publicKey":         publicKey,
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(keyUnknownProvider, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWS_NilImportParameters", func(t *testing.T) {
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: nil}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AWSEmptyFields", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) {
			k.Provider = providerTest
		})
		fields := map[string]any{
			"publicKey":         "",
			"wrappingAlgorithm": wrappingAlgorithm,
			"hashFunction":      hashFunction,
			"providerParams":    providerParams,
			"validTo":           validTimeStr,
		}
		structData, _ := structpb.NewStruct(fields)
		response := &keystoreopv1.GetImportParametersResponse{ImportParameters: structData}

		result, err := manager.BuildImportParams(key, response)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
