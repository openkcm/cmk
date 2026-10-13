package key_management_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	keystoreErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	wrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/key_management"
)

var errUnknown = errors.New("some unknown error")

func TestConvertGRPCError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "nil",
			err:     nil,
			wantErr: nil,
		},
		{
			name:    "ProviderAuthenticationError",
			err:     keystoreErrs.StatusProviderAuthenticationError.Err(),
			wantErr: keymanagement.ErrProviderAuthenticationFailed,
		},
		{
			name:    "KeyNotFound",
			err:     keystoreErrs.StatusKeyNotFound.Err(),
			wantErr: keymanagement.ErrHYOKKeyNotFound,
		},
		{
			name:    "KeyGenericErr",
			err:     keystoreErrs.StatusKeyGenericErr.Err(),
			wantErr: keymanagement.ErrGenericGetKeyError,
		},
		{
			name: "ImportKeyMaterialFailed",
			err: status.New(codes.InvalidArgument,
				"key material decryption failed: invalid or incorrectly wrapped key material").Err(),
			wantErr: keymanagement.ErrImportKeyMaterialFailed,
		},
		{
			name:    "UnknownError_passthrough",
			err:     errUnknown,
			wantErr: errUnknown,
		},
		{
			name:    "UnmappedGRPCStatus_passthrough",
			err:     status.New(codes.Internal, "some internal error").Err(),
			wantErr: status.New(codes.Internal, "some internal error").Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapper.ConvertGRPCError(tt.err)

			if tt.wantErr == nil {
				assert.NoError(t, got)
				return
			}
			assert.ErrorIs(t, got, tt.wantErr)
		})
	}
}

func TestConvertGRPCError_ImportKeyMaterialFailed_PreservesProviderError(t *testing.T) {
	providerErr := status.New(codes.InvalidArgument,
		"key material decryption failed: invalid or incorrectly wrapped key material").Err()

	got := wrapper.ConvertGRPCError(providerErr)

	assert.ErrorIs(t, got, keymanagement.ErrImportKeyMaterialFailed)
	assert.ErrorIs(t, got, providerErr)
}
