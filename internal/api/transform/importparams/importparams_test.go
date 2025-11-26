package importparams_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/importparams"
	"github.com/openkcm/cmk/internal/model"
)

func TestToAPI(t *testing.T) {
	p := model.ImportParams{
		PublicKeyPEM: "test-public-key",
		WrappingAlg:  "RSA-OAEP",
		HashFunction: "SHA-256",
	}

	api, err := importparams.ToAPI(p)
	assert.NoError(t, err)

	assert.NotNil(t, api)
	assert.Equal(t, "test-public-key", *api.PublicKey)
	assert.NotNil(t, api.WrappingAlgorithm)
	assert.Equal(t, cmkapi.WrappingAlgorithmName("RSA-OAEP"), api.WrappingAlgorithm.Name)
	assert.Equal(t, cmkapi.WrappingAlgorithmHashFunction("SHA-256"), api.WrappingAlgorithm.HashFunction)
}

func TestToAPI_EmptyFields(t *testing.T) {
	p := model.ImportParams{}
	api, err := importparams.ToAPI(p)
	assert.NoError(t, err)

	assert.NotNil(t, api)
	assert.NotNil(t, api.PublicKey)
	assert.Empty(t, *api.PublicKey)
	assert.NotNil(t, api.WrappingAlgorithm)
	assert.Equal(t, cmkapi.WrappingAlgorithmName(""), api.WrappingAlgorithm.Name)
	assert.Equal(t, cmkapi.WrappingAlgorithmHashFunction(""), api.WrappingAlgorithm.HashFunction)
}
