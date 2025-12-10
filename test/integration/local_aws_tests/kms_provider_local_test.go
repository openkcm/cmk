//go:build !unit

package localaws_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/providers"
	"github.tools.sap/kms/cmk/providers/clients/aws"
	"github.tools.sap/kms/cmk/utils/ptr"
)

// Local KMS variables
var (
	externalID = "0000-0000-0000-0000-0001"
	unknownKey = &providers.Key{
		ID:          ptr.PointTo("unknown-key-id"),
		KeyVersions: []providers.KeyVersion{{ExternalID: &externalID}},
	}
	localKms = providers.NewProvider(aws.NewBaseEndpointClient(context.Background(),
		"us-west-2", "http://localhost:8081"))
	invalidKms = providers.NewProvider(aws.NewBaseEndpointClient(context.Background(),
		"us-west-2", "http://dummy:8080"))
)

// TestCreateKey tests the CreateKey method of the KMSService.
func TestCreateKey(t *testing.T) {
	tests := []struct {
		name     string
		provider *providers.Provider
		wantErr  bool
	}{
		{
			name:     "CreateKeySuccess",
			provider: localKms,
			wantErr:  false,
		},
		{
			name:     "CreateKeyError",
			provider: invalidKms,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.provider.CreateKey(
				t.Context(),
				providers.KeyInput{ID: ptr.PointTo(uuid.New().String()), KeyType: providers.AES256},
			)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDeleteKey tests the DeleteKey method of the KMSService.
func TestDeleteKey(t *testing.T) {
	tests := []struct {
		name    string
		key     *providers.Key
		wantErr bool
	}{
		{
			name:    "DeleteKeySuccess",
			key:     nil,
			wantErr: false,
		},
		{
			name:    "DeleteKeyError",
			key:     unknownKey,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key == nil {
				tt.key, _ = localKms.CreateKey(
					t.Context(),
					providers.KeyInput{
						ID:      ptr.PointTo(uuid.New().String()),
						KeyType: providers.AES256,
					},
				)
			}

			err := localKms.DeleteKey(t.Context(), tt.key, providers.DeleteOptions{})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEnableKey tests the EnableKey method of the KMSService.
func TestEnableKey(t *testing.T) {
	tests := []struct {
		name    string
		key     *providers.Key
		wantErr bool
	}{
		{
			name:    "EnableKeySuccess",
			key:     nil,
			wantErr: false,
		},
		{
			name:    "EnableKeyError",
			key:     unknownKey,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key == nil {
				tt.key, _ = localKms.CreateKey(
					t.Context(),
					providers.KeyInput{
						ID:      ptr.PointTo(uuid.New().String()),
						KeyType: providers.AES256,
					},
				)
			}

			err := localKms.EnableKey(t.Context(), tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDisableKey tests the DisableKey method of the KMSService.
func TestDisableKey(t *testing.T) {
	tests := []struct {
		name    string
		key     *providers.Key
		wantErr bool
	}{
		{
			name:    "DisableKeySuccess",
			key:     nil,
			wantErr: false,
		},
		{
			name:    "DisableKeyError",
			key:     unknownKey,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key == nil {
				tt.key, _ = localKms.CreateKey(
					t.Context(),
					providers.KeyInput{
						ID:      ptr.PointTo(uuid.New().String()),
						KeyType: providers.AES256,
					},
				)
			}

			err := localKms.DisableKey(t.Context(), tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRotateKey tests the RotateKey method of the KMSService.
func TestRotateKey(t *testing.T) {
	tests := []struct {
		name          string
		key           *providers.Key
		wantErr       bool
		versionsCount int
	}{
		{
			name:          "RotateKeySuccess",
			key:           nil,
			wantErr:       false,
			versionsCount: 2,
		},
		{
			name:          "RotateKeyError",
			key:           unknownKey,
			wantErr:       true,
			versionsCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key == nil {
				tt.key, _ = localKms.CreateKey(
					t.Context(),
					providers.KeyInput{
						ID:      ptr.PointTo(uuid.New().String()),
						KeyType: providers.AES256,
					},
				)
			}

			err := localKms.RotateKey(t.Context(), tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, tt.key.KeyVersions, tt.versionsCount)
			}
		})
	}
}
