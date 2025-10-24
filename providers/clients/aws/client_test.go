package aws_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/providers"
	"github.com/openkcm/cmk-core/providers/clients/aws"
	"github.com/openkcm/cmk-core/providers/clients/aws/mock"
	"github.com/openkcm/cmk-core/utils/ptr"
)

var (
	key            = uuid.New().String()
	secret         = uuid.New().String()
	token          = uuid.New().String()
	region         = uuid.New().String()
	errForced      = errors.New("forced error")
	expectedID     = uuid.New().String()
	expectedKeyID  = uuid.New().String()
	now            = time.Now()
	expectedKeyArn = "arn:aws:kms:us-west-2:123456789012:key/" + expectedKeyID
	aliasName      = aws.PrepareAlias(expectedID)
	happyPathMock  = mock.HappyPathMock(expectedKeyID, expectedKeyArn, now)
	errorMock      = mock.ErrorMock(errForced)
	baseEndpoint   = "https://kms.us-west-2.amazonaws.com"
)

// TestClient_CreateNativeKey tests the CreateKeyVersion function
func TestClient_CreateNativeKey(t *testing.T) {
	tests := []struct {
		name    string
		client  *mock.Mock
		options providers.KeyInput
		wantErr error
		wantArn *string
	}{
		{
			name:    "CreateNativeKey_Success",
			client:  happyPathMock,
			options: providers.KeyInput{ID: &expectedID, KeyType: providers.AES256},
			wantErr: nil,
			wantArn: &expectedKeyArn,
		},
		{
			name:    "CreateNativeKey_MissingIDError",
			client:  happyPathMock,
			options: providers.KeyInput{},
			wantErr: aws.ErrMissingID,
		},
		{
			name:    "CreateNativeKey_CreateKeyError",
			client:  errorMock,
			options: providers.KeyInput{ID: &expectedID, KeyType: providers.AES256},
			wantErr: aws.ErrNativeCreateKeyFailed,
			wantArn: nil,
		},
		{
			name:    "CreateNativeKey_WrongKeyInputError",
			client:  happyPathMock,
			options: providers.KeyInput{ID: &expectedID, KeyType: "wrong"},
			wantErr: aws.ErrNativeCreateKeyFailedWrongKeyInput,
			wantArn: nil,
		},
		{
			name: "CreateNativeKey_CreateAliasError",
			client: mock.HappyPathMock(expectedKeyID, expectedKeyArn, now).WithCreateAliasFunc(
				func(_ context.Context,
					_ *kms.CreateAliasInput,
					_ ...func(*kms.Options)) (*kms.CreateAliasOutput, error) {
					return nil, errForced
				},
			),
			options: providers.KeyInput{ID: &expectedID, KeyType: providers.AES256},
			wantErr: aws.ErrNativeCreateAliasFailed,
			wantArn: nil,
		},
		{
			name: "CreateNativeKey_UpdateAliasError",
			client: mock.HappyPathMock(expectedKeyID, expectedKeyArn, now).
				WithUpdateAliasFunc(
					func(_ context.Context,
						_ *kms.UpdateAliasInput,
						_ ...func(*kms.Options)) (*kms.UpdateAliasOutput, error) {
						return nil, errForced
					}),
			options: providers.KeyInput{ID: &expectedID, KeyType: providers.AES256},
			wantErr: aws.ErrNativeUpdateAliasFailed,
			wantArn: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := aws.NewClientForTests(tt.client)

			arn, err := c.CreateKeyVersion(t.Context(), tt.options)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantArn, arn)
		})
	}
}

// testAlias is a helper function to test alias-related functions
func testAlias(
	t *testing.T,
	testName string,
	actionFunc func(c *aws.Client, aliasName, nativeKeyID string) error,
) {
	t.Helper()

	tests := []struct {
		name        string
		client      *mock.Mock
		nativeKeyID string
		wantErr     error
	}{
		{
			name:        testName + "_Success",
			client:      happyPathMock,
			nativeKeyID: expectedKeyID,
			wantErr:     nil,
		},
		{
			name:        testName + "_Error",
			client:      errorMock,
			nativeKeyID: expectedKeyID,
			wantErr:     errForced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := aws.NewClientForTests(tt.client)

			err := actionFunc(c, aliasName, tt.nativeKeyID)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// testFunction tests the native function
func testFunction(
	t *testing.T,
	testName string,
	actionFunc func(c *aws.Client, keyID string) error,
) {
	t.Helper()

	tests := []struct {
		name        string
		client      *mock.Mock
		nativeKeyID string
		wantErr     error
	}{
		{
			name:        testName + "_Success",
			client:      happyPathMock,
			nativeKeyID: expectedKeyID,
			wantErr:     nil,
		},
		{
			name:        testName + "_Error",
			client:      errorMock,
			nativeKeyID: expectedKeyID,
			wantErr:     errForced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := aws.NewClientForTests(tt.client)

			err := actionFunc(c, tt.nativeKeyID)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClient_CreateNativeAlias tests the CreateNativeAlias function
func TestClient_CreateNativeAlias(t *testing.T) {
	testAlias(t, "CreateNativeAlias", func(c *aws.Client, aliasName, nativeKeyID string) error {
		return c.ExportCreateAlias()(t.Context(), aliasName, nativeKeyID)
	})
}

// TestClient_UpdateNativeAlias tests the UpdateNativeAlias function
func TestClient_UpdateNativeAlias(t *testing.T) {
	testAlias(t, "UpdateNativeAlias", func(c *aws.Client, aliasName, nativeKeyID string) error {
		return c.ExportUpdateAlias()(t.Context(), aliasName, nativeKeyID)
	})
}

// TestClient_EnsureAlias tests the EnsureAlias function
func TestClient_EnsureAlias(t *testing.T) {
	tests := []struct {
		name    string
		client  *mock.Mock
		keyID   string
		id      string
		wantErr error
	}{
		{
			name:    "EnsureAlias_Success_CreateAlias",
			client:  happyPathMock,
			keyID:   expectedKeyID,
			id:      expectedID,
			wantErr: nil,
		},
		{
			name: "EnsureAlias_Success_UpdateAlias",
			client: mock.HappyPathMock(expectedKeyID, expectedKeyArn, now).
				WithCreateAliasFunc(
					func(_ context.Context,
						_ *kms.CreateAliasInput,
						_ ...func(*kms.Options)) (*kms.CreateAliasOutput, error) {
						return nil,
							&types.AlreadyExistsException{}
					}),
			keyID:   expectedKeyID,
			id:      expectedID,
			wantErr: nil,
		},
		{
			name: "EnsureAlias_Error_CreateAlias",
			client: mock.HappyPathMock(expectedKeyID, expectedKeyArn, now).
				WithCreateAliasFunc(
					func(_ context.Context,
						_ *kms.CreateAliasInput,
						_ ...func(*kms.Options)) (*kms.CreateAliasOutput, error) {
						return nil, errForced
					}),
			keyID:   expectedKeyID,
			id:      expectedID,
			wantErr: aws.ErrNativeCreateAliasFailed,
		},
		{
			name: "EnsureAlias_Error_UpdateAlias",
			client: mock.HappyPathMock(expectedKeyID, expectedKeyArn, now).
				WithCreateAliasFunc(
					func(_ context.Context,
						_ *kms.CreateAliasInput,
						_ ...func(*kms.Options)) (*kms.CreateAliasOutput, error) {
						return nil,
							&types.AlreadyExistsException{}
					}).
				WithUpdateAliasFunc(
					func(_ context.Context,
						_ *kms.UpdateAliasInput,
						_ ...func(*kms.Options)) (*kms.UpdateAliasOutput, error) {
						return nil, errForced
					}),
			keyID:   expectedKeyID,
			id:      expectedID,
			wantErr: aws.ErrNativeUpdateAliasFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := aws.NewClientForTests(tt.client)

			err := c.ExportEnsureAlias()(t.Context(), tt.keyID, tt.id)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClient_EnableNative tests the EnableKeyVersion function
func TestClient_EnableNative(t *testing.T) {
	testFunction(t, "EnableKeyVersion", func(c *aws.Client, keyID string) error {
		return c.EnableKeyVersion(t.Context(), keyID)
	})
}

// TestClient_DisableNative tests the DisableKeyVersion function
func TestClient_DisableNative(t *testing.T) {
	testFunction(t, "DisableKeyVersion", func(c *aws.Client, keyID string) error {
		return c.DisableKeyVersion(t.Context(), keyID)
	})
}

// TestClient_DeleteNative tests the DeleteKeyVersion function
func TestClient_DeleteNative(t *testing.T) {
	testFunction(t, "DeleteKeyVersion", func(c *aws.Client, keyID string) error {
		return c.DeleteKeyVersion(t.Context(), keyID, providers.DeleteOptions{})
	})
}

// TestCreateKeyInputFromKeyOptions tests the createKeyInputFromKeyOptions function
func TestCreateKeyInputFromKeyOptions(t *testing.T) {
	tests := []struct {
		name    string
		options providers.KeyInput
		want    *kms.CreateKeyInput
		wantErr error
	}{
		{
			name:    "CreateKeyInputFromKeyOptions_AES256",
			options: providers.KeyInput{KeyType: providers.AES256},
			want: &kms.CreateKeyInput{
				KeySpec:  types.KeySpecSymmetricDefault,
				KeyUsage: types.KeyUsageTypeEncryptDecrypt,
				Origin:   types.OriginTypeAwsKms,
			},
			wantErr: nil,
		},
		{
			name:    "CreateKeyInputFromKeyOptions_RSA3072",
			options: providers.KeyInput{KeyType: providers.RSA3072},
			want: &kms.CreateKeyInput{
				KeySpec:  types.KeySpecRsa3072,
				KeyUsage: types.KeyUsageTypeEncryptDecrypt,
				Origin:   types.OriginTypeAwsKms,
			},
			wantErr: nil,
		},
		{
			name:    "CreateKeyInputFromKeyOptions_RSA4096",
			options: providers.KeyInput{KeyType: providers.RSA4096},
			want: &kms.CreateKeyInput{
				KeySpec:  types.KeySpecRsa4096,
				KeyUsage: types.KeyUsageTypeEncryptDecrypt,
				Origin:   types.OriginTypeAwsKms,
			},
			wantErr: nil,
		},
		{
			name:    "CreateKeyInputFromKeyOptions_UnknownKeyType",
			options: providers.KeyInput{KeyType: "unknown"},
			want:    nil,
			wantErr: fmt.Errorf("%w: %v", aws.ErrUnknownOptionType, "unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := aws.CreateKeyInputFromKeyOptions(tt.options)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, result)
		})
	}
}

// TestCreateKeyFromCreateKeyOutput tests the CreateKeyFromCreateKeyOutput function
func TestCreateScheduleKeyDeletionInputFromDeleteOptions(t *testing.T) {
	tests := []struct {
		name    string
		keyID   string
		options providers.DeleteOptions
		want    *kms.ScheduleKeyDeletionInput
	}{
		{
			name:  "CreateScheduleKeyDeletionInput_WindowSet",
			keyID: expectedKeyID,
			options: providers.DeleteOptions{
				Window: ptr.PointTo(int32(7)),
			},
			want: &kms.ScheduleKeyDeletionInput{
				KeyId:               &expectedKeyID,
				PendingWindowInDays: ptr.PointTo(int32(7)),
			},
		},
		{
			name:    "CreateScheduleKeyDeletionInput_WindowNotSet",
			keyID:   expectedKeyID,
			options: providers.DeleteOptions{},
			want: &kms.ScheduleKeyDeletionInput{
				KeyId: &expectedKeyID,
			},
		},
		{
			name:    "CreateScheduleKeyDeletionInput_OptionsNil",
			keyID:   expectedKeyID,
			options: providers.DeleteOptions{},
			want: &kms.ScheduleKeyDeletionInput{
				KeyId: &expectedKeyID,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aws.CreateScheduleKeyDeletionInputFromDeleteOptions(tt.keyID, tt.options)
			assert.Equal(t, tt.want, result)
		})
	}
}

// Helper function to validate internal client and aws_options
func validateClient(
	t *testing.T,
	client *aws.Client,
	region, key, secret string,
	baseEndpoint ...*string,
) {
	t.Helper()

	assert.NotNil(t, client)

	internalClient := client.ExportInternalClientForTests(t)

	internalOptions := internalClient.Options()
	assert.Equal(t, region, internalOptions.Region)

	retrieve, err := internalOptions.Credentials.Retrieve(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, key, retrieve.AccessKeyID)
	assert.Equal(t, secret, retrieve.SecretAccessKey)

	if len(baseEndpoint) > 0 && baseEndpoint[0] != nil {
		assert.Equal(t, baseEndpoint[0], internalOptions.BaseEndpoint)
	}
}

// TestNewClientWithOptions tests the NewClientWithOptions function to ensure it creates a valid KMS client.
func TestNewClientWithOptions(t *testing.T) {
	ctx := t.Context()
	credentialsProvider := credentials.NewStaticCredentialsProvider(key, secret, token)

	client := aws.NewClientWithOptions(
		ctx,
		region,
		credentialsProvider,
		aws.BaseEndpoint(baseEndpoint),
	)

	validateClient(t, client, region, key, secret, &baseEndpoint)
}

// TestNewClient tests the NewClient function to ensure it creates a valid KMS client with static credentials.
func TestNewClient(t *testing.T) {
	ctx := t.Context()

	client := aws.NewClient(ctx, region, key, secret)

	validateClient(t, client, region, key, secret)
}

// TestNewClientFromCredentialsProvider tests the NewClientFromCredentialsProvider
// function to ensure it creates a valid KMS client from a credentials providers.
func TestNewClientFromCredentialsProvider(t *testing.T) {
	ctx := t.Context()
	credentialsProvider := credentials.NewStaticCredentialsProvider(key, secret, token)

	client := aws.NewClientFromCredentialsProvider(ctx, region, credentialsProvider)

	validateClient(t, client, region, key, secret)
}

// TestNewBaseEndpointClient tests the NewBaseEndpointClient
// function to ensure it creates a valid KMS client with a base endpoint.
func TestNewBaseEndpointClient(t *testing.T) {
	ctx := t.Context()

	client := aws.NewBaseEndpointClient(ctx, region, baseEndpoint)

	validateClient(t, client, region, "dummy", "dummy", &baseEndpoint)
}
