package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cryptoUtils "github.com/openkcm/cmk/utils/crypto"
)

func TestGeneratePrivateKey(t *testing.T) {
	var tests = []struct {
		name        string
		bitSize     int
		expectedErr bool
	}{
		{
			name:        "GeneratePrivateKey_SUCCESS",
			bitSize:     2048,
			expectedErr: false,
		},
		{
			name:        "GeneratePrivateKey_ERROR",
			bitSize:     1,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey, err := cryptoUtils.GeneratePrivateKey(tt.bitSize)
			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, privateKey)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, privateKey)
			}
		})
	}
}
