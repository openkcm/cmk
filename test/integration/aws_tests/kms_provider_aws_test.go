//go:build !unit

package aws_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/providers"
	"github.com/openkcm/cmk/providers/clients/aws"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	EuNorth1      = "eu-north-1"
	LocalEndpoint = "http://localhost:8081"
)

var ErrDescribeKey = errors.New("error describe key")

// ensureProvider checks if the AWS accessCredentials are provided.
func accessCredentials(t *testing.T) (string, string) {
	t.Helper()

	return "dummy", "dummy"
}

// ensureKmsClient ensures that proper KMS client is returned.
func ensureKmsClient(t *testing.T) *kms.Client {
	t.Helper()

	accessKeyID, secretAccessKey := accessCredentials(t)
	credentialsProvider := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
	loadOptions := []func(*config.LoadOptions) error{
		config.WithRegion(EuNorth1),
		config.WithCredentialsProvider(credentialsProvider),
	}
	cfg, _ := config.LoadDefaultConfig(t.Context(), loadOptions...)

	return kms.NewFromConfig(cfg, aws.BaseEndpoint(LocalEndpoint))
}

// ensureProvider ensures that proper provider is returned.
func ensureProvider() func(t *testing.T) *providers.Provider {
	var provider *providers.Provider

	inlineFunc := func(t *testing.T) *providers.Provider {
		t.Helper()

		if provider != nil {
			return provider
		}

		provider = providers.NewProvider(aws.NewBaseEndpointClient(t.Context(),
			EuNorth1, LocalEndpoint))

		return provider
	}

	return inlineFunc
}

// TestCreateKey tests the CreateKey method of the KMSService.
func TestKeyStateChanges(t *testing.T) {
	awsKms := ensureProvider()(t)

	var key *providers.Key

	var err error

	t.Run("Provider Aws Integration Test - CreateKey Test", func(t *testing.T) {
		key, err = awsKms.CreateKey(
			t.Context(),
			providers.KeyInput{ID: ptr.PointTo(uuid.New().String()), KeyType: providers.AES256},
		)

		assert.NoError(t, err)
		assert.Len(t, key.KeyVersions, 1)

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.ENABLED, err)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStateEnabled)
	})
	t.Run("Provider Aws Integration Test - DisableKey Test", func(t *testing.T) {
		disableErr := awsKms.DisableKey(t.Context(), key)

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.DISABLED, disableErr)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStateDisabled)

		enableErr := awsKms.EnableKey(t.Context(), key)

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.ENABLED, enableErr)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStateEnabled)
	})
	t.Run("Provider Aws Integration Test - RotateKey Test", func(t *testing.T) {
		rotateErr := awsKms.RotateKey(t.Context(), key)

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.ENABLED, rotateErr)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStateEnabled)
		AssertLocalKeyStateByID(t, key.KeyVersions[1], providers.ENABLED, rotateErr)
		AssertAwsKeyStateByID(t, key.KeyVersions[1].ExternalID, types.KeyStateEnabled)
		assert.Len(t, key.KeyVersions, 2)
	})
	t.Run("Provider Aws Integration Test - DisableKey Test", func(t *testing.T) {
		disableErr2 := awsKms.DisableKey(t.Context(), key)

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.DISABLED, disableErr2)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStateDisabled)
		AssertLocalKeyStateByID(t, key.KeyVersions[1], providers.DISABLED, disableErr2)
		AssertAwsKeyStateByID(t, key.KeyVersions[1].ExternalID, types.KeyStateDisabled)
	})
	t.Run("Provider Aws Integration Test - EnableKey Test", func(t *testing.T) {
		enableErr2 := awsKms.EnableKey(t.Context(), key)

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.DISABLED, enableErr2)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStateDisabled)
		AssertLocalKeyStateByID(t, key.KeyVersions[1], providers.ENABLED, enableErr2)
		AssertAwsKeyStateByID(t, key.KeyVersions[1].ExternalID, types.KeyStateEnabled)
	})
	t.Run("Provider Aws Integration Test - DeleteKey Test", func(t *testing.T) {
		deleteErr := awsKms.DeleteKey(t.Context(), key, providers.DeleteOptions{})

		AssertLocalKeyStateByID(t, key.KeyVersions[0], providers.DELETED, deleteErr)
		AssertAwsKeyStateByID(t, key.KeyVersions[0].ExternalID, types.KeyStatePendingDeletion)
		AssertLocalKeyStateByID(t, key.KeyVersions[1], providers.DELETED, deleteErr)
		AssertAwsKeyStateByID(t, key.KeyVersions[1].ExternalID, types.KeyStatePendingDeletion)
		assert.Len(t, key.KeyVersions, 2)
	})
}

// DescribeKeyByID describes a key by its external ID.
func DescribeKeyByID(t *testing.T, externalID *string) (*kms.DescribeKeyOutput, error) {
	t.Helper()
	kmsClient := ensureKmsClient(t)

	describeKeyInput := kms.DescribeKeyInput{
		KeyId: externalID,
	}

	describeKey, err := kmsClient.DescribeKey(t.Context(), &describeKeyInput)
	if err != nil {
		return nil, fmt.Errorf("%w,%w", ErrDescribeKey, err)
	}

	return describeKey, nil
}

// AssertLocalKeyStateByID asserts the local key state by its ID.
func AssertLocalKeyStateByID(
	t *testing.T,
	keyVersion providers.KeyVersion,
	state providers.KeyState,
	err error,
) {
	t.Helper()

	assert.NoError(t, err)
	assert.Equal(t, state, keyVersion.State)
}

// AssertAwsKeyStateByID asserts the AWS key state by its external ID.
func AssertAwsKeyStateByID(t *testing.T, externalID *string, keyState types.KeyState) {
	t.Helper()

	describeKeyOutput, describeKeyErr := DescribeKeyByID(t, externalID)
	assert.NoError(t, describeKeyErr)
	assert.NotNil(t, describeKeyOutput)
	assert.NotEmpty(t, describeKeyOutput.KeyMetadata)
	assert.Equal(t, keyState, describeKeyOutput.KeyMetadata.KeyState)
}
