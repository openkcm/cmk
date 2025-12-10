package providers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/providers"
	"github.tools.sap/kms/cmk/providers/mock"
	"github.tools.sap/kms/cmk/utils/ptr"
	"github.tools.sap/kms/cmk/utils/slice"
)

var (
	testsStartTime = time.Now()
	expectedKeyID  = ptr.PointTo(uuid.New().String())
	expectedID     = uuid.New().String()
	errForced      = errors.New("error")
	createdTime    = ptr.PointTo(time.Now().Add(-time.Hour))
	secondKeyID    = ptr.PointTo(uuid.New().String())
	thirdKeyID     = ptr.PointTo(uuid.New().String())
)

// TestProvider_CreateKeyVersion - checks if CreateKeyVersion of Provider works as expected
func TestProvider_CreateKeyVersion(t *testing.T) {
	tests := []struct {
		name    string
		client  providers.Client
		want    *providers.Key
		wantErr error
		options providers.KeyInput
	}{
		{
			name: "CreateKeyVersionSuccessChosenKey",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return expectedKeyID, nil
				}),
			options: providers.KeyInput{KeyType: providers.RSA3072},
			want: &providers.Key{
				KeyVersions: []providers.KeyVersion{{
					ExternalID: expectedKeyID,
					State:      providers.ENABLED,
					Version:    1,
				}},
				KeyType: providers.RSA3072,
			},
		},
		{
			name: "CreateKeyVersionError",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return nil, errForced
				}),
			want:    nil,
			wantErr: providers.ErrCreateKeyVersionFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := providers.NewProvider(tt.client)

			got, err := c.ExportCreateKeyVersion()(t.Context(), tt.options, 1)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.KeyVersions[0].ExternalID, got.ExternalID)
				checkTimeProximity(t, testsStartTime, *got.CreatedAt)
			}
		})
	}
}

// TestAWSClient_CreateKey tests the CreateKey function of the Provider struct.
func TestAWSClient_CreateKey(t *testing.T) {
	tests := []struct {
		name         string
		client       providers.Client
		wantErrChain []error
		want         *providers.Key
	}{
		{
			name: "Success",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return expectedKeyID, nil
				}),
			want: &providers.Key{
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: expectedKeyID,
						State:      providers.ENABLED,
						Version:    1,
					},
				},
				KeyType: providers.AES256,
			},
		},
		{
			name: "CreateKeyError",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return nil, errForced
				}),
			wantErrChain: []error{
				providers.ErrCreateKeyFailed,
				providers.ErrCreateKeyVersionFailed,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := providers.NewProvider(tt.client)
			got, err := c.CreateKey(
				t.Context(),
				providers.KeyInput{KeyType: providers.AES256, ID: &expectedID},
			)

			if len(tt.wantErrChain) > 0 {
				for _, e := range tt.wantErrChain {
					assert.ErrorIs(t, err, e)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.KeyVersions[0].ExternalID, got.KeyVersions[0].ExternalID)
				assert.Equal(t, tt.want.KeyType, got.KeyType)
				checkTimeProximity(t, testsStartTime, *got.KeyVersions[0].CreatedAt)
			}
		})
	}
}

// TestClient_RotateKey - checks if RotateKey of client works as expected
func TestClient_RotateKey(t *testing.T) {
	tests := []struct {
		name    string
		client  providers.Client
		key     *providers.Key
		wantErr error
		want    *providers.Key
	}{
		{
			name: "RotateKey_Success",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return secondKeyID, nil
				}),
			key: &providers.Key{
				ID:      &expectedID,
				Version: 1,
				KeyType: providers.AES256,
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: expectedKeyID,
						State:      providers.ENABLED,
						Version:    1,
						CreatedAt:  createdTime,
					},
				},
			},
			want: &providers.Key{
				ID:      &expectedID,
				Version: 2,
				KeyType: providers.AES256,
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: expectedKeyID,
						State:      providers.ENABLED,
						Version:    1,
						CreatedAt:  createdTime,
					},
					{
						ExternalID: secondKeyID,
						State:      providers.ENABLED,
						Version:    2,
						CreatedAt:  &testsStartTime,
					},
				},
			},
		},
		{
			name: "RotateKey_Success_WithMultipleVersions",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return thirdKeyID, nil
				}),
			key: &providers.Key{
				ID:      &expectedID,
				KeyType: providers.AES256,
				Version: 2,
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: expectedKeyID,
						State:      providers.ENABLED,
						Version:    1,
						CreatedAt:  createdTime,
					},
					{
						ExternalID: secondKeyID,
						State:      providers.ENABLED,
						Version:    2,
						CreatedAt:  createdTime,
					},
				},
			},
			want: &providers.Key{
				ID:      &expectedID,
				Version: 3,
				KeyType: providers.AES256,
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: expectedKeyID,
						State:      providers.ENABLED,
						Version:    1,
						CreatedAt:  createdTime,
					},
					{
						ExternalID: secondKeyID,
						State:      providers.ENABLED,
						Version:    2,
						CreatedAt:  createdTime,
					},
					{
						ExternalID: thirdKeyID,
						State:      providers.ENABLED,
						Version:    3,
						CreatedAt:  &testsStartTime,
					},
				},
			},
		},
		{
			name: "RotateKey_CreateKeyError",
			client: mock.NewClientMock().WithCreateNativeKey(
				func(_ context.Context, _ providers.KeyInput) (*string, error) {
					return nil, errForced
				}),
			key: &providers.Key{
				KeyVersions: []providers.KeyVersion{{
					ExternalID: expectedKeyID,
					State:      providers.ENABLED,
				}},
				KeyType: providers.AES256,
			},
			wantErr: providers.ErrRotateKeyFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := providers.NewProvider(tt.client)

			err := c.RotateKey(t.Context(), tt.key)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)

				version := slice.LastElement(tt.key.KeyVersions)
				checkTimeProximity(t, testsStartTime, *version.CreatedAt)

				tt.key.KeyVersions[len(tt.key.KeyVersions)-1].CreatedAt = tt.want.KeyVersions[len(tt.key.KeyVersions)-1].CreatedAt

				assert.Equal(t, tt.want, tt.key)
			}
		})
	}
}

// TestAWSClient_EnableKey - checks if EnableKey of Provider works as expected
func TestAWSClient_EnableKey(t *testing.T) {
	tests := []struct {
		name    string
		client  providers.Client
		wantErr error
	}{
		{
			name: "EnableKeySuccess",
			client: mock.NewClientMock().WithEnableNative(
				func(_ context.Context, _ string) error {
					return nil
				}),
		},
		{
			name: "EnableKeyError",
			client: mock.NewClientMock().WithEnableNative(
				func(_ context.Context, _ string) error {
					return errForced
				}),
			wantErr: providers.ErrEnableKeyFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := providers.NewProvider(tt.client)
			key := &providers.Key{
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    1,
						State:      providers.DISABLED,
					},
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    2,
						State:      providers.ERROR,
					},
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    3,
						State:      providers.DISABLED,
					},
				},
				Version: 3,
				KeyType: providers.AES256,
			}

			err := c.EnableKey(t.Context(), key)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, providers.ERROR, slice.LastElement(key.KeyVersions).State)
			} else {
				checkTimeProximity(t, testsStartTime, *slice.LastElement(key.KeyVersions).UpdatedAt)

				for i, kv := range key.KeyVersions {
					if i == len(key.KeyVersions)-1 {
						assert.Equal(t, providers.ENABLED, kv.State)
					} else {
						assert.NotEqual(t, providers.ENABLED, kv.State)
					}
				}
			}
		})
	}
}

// TestAWSClient_DisableKey - checks if DisableKey of Provider works as expected
func TestAWSClient_DisableKey(t *testing.T) {
	tests := []struct {
		name    string
		client  providers.Client
		wantErr error
	}{
		{
			name: "DisableKeySuccess",
			client: mock.NewClientMock().WithDisableNative(
				func(_ context.Context, _ string) error {
					return nil
				}),
		},
		{
			name: "DisableKey_Error",
			client: mock.NewClientMock().WithDisableNative(
				func(_ context.Context, _ string) error {
					return errForced
				}),
			wantErr: providers.ErrDisableKeyFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := providers.NewProvider(tt.client)
			key := &providers.Key{
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    1,
						State:      providers.ENABLED,
					},
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    2,
						State:      providers.ENABLED,
					},
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    3,
						State:      providers.ENABLED,
					},
				},
				Version: 3,
				KeyType: providers.AES256,
			}

			err := c.DisableKey(t.Context(), key)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, providers.ERROR, key.KeyVersions[0].State)
			} else {
				checkTimeProximity(t, testsStartTime, *slice.LastElement(key.KeyVersions).UpdatedAt)

				for _, kv := range key.KeyVersions {
					assert.Equal(t, providers.DISABLED, kv.State)
				}
			}
		})
	}
}

// TestAWSClient_DeleteKey - checks if DeleteKey of Provider works as expected
func TestAWSClient_DeleteKey(t *testing.T) {
	tests := []struct {
		name      string
		client    providers.Client
		wantErr   error
		wantState providers.KeyState
	}{
		{
			name: "DeleteKeySuccess",
			client: mock.NewClientMock().WithDeleteNative(
				func(_ context.Context, _ string, _ providers.DeleteOptions) error {
					return nil
				}),
			wantState: providers.DELETED,
		},
		{
			name: "DeleteKeyError",
			client: mock.NewClientMock().WithDeleteNative(
				func(_ context.Context, _ string, _ providers.DeleteOptions) error {
					return errForced
				}),
			wantErr:   providers.ErrDeleteKeyFailed,
			wantState: providers.ERROR,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := providers.NewProvider(tt.client)
			key := &providers.Key{
				KeyVersions: []providers.KeyVersion{
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    1,
						State:      providers.ENABLED,
					},
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    2,
						State:      providers.DISABLED,
					},
					{
						ExternalID: ptr.PointTo(uuid.New().String()),
						Version:    3,
						State:      providers.ERROR,
					},
				},
				Version: 3,
				KeyType: providers.AES256,
			}

			err := c.DeleteKey(t.Context(), key, providers.DeleteOptions{})

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, providers.ERROR, key.KeyVersions[0].State)
			} else {
				checkTimeProximity(t, testsStartTime, *slice.LastElement(key.KeyVersions).UpdatedAt)

				for _, kv := range key.KeyVersions {
					assert.Equal(t, providers.DELETED, kv.State)
				}
			}
		})
	}
}

// checkTimeProximity - helper function to evaluate time proximity
func checkTimeProximity(t *testing.T, now, got time.Time) {
	t.Helper()
	assert.WithinDuration(t, now, got, 2*time.Second)
}
