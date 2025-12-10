package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/providers"
	"github.tools.sap/kms/cmk/utils/must"
	"github.tools.sap/kms/cmk/utils/ptr"
)

// kmsClient defines the methods of the AWS KMS client that we use.
// This is used for mocking the client in tests.
type kmsClient interface {
	CreateKey(ctx context.Context,
		params *kms.CreateKeyInput,
		optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	EnableKey(ctx context.Context,
		params *kms.EnableKeyInput,
		optFns ...func(*kms.Options)) (*kms.EnableKeyOutput, error)
	DisableKey(ctx context.Context,
		params *kms.DisableKeyInput,
		optFns ...func(*kms.Options)) (*kms.DisableKeyOutput, error)
	ScheduleKeyDeletion(ctx context.Context,
		params *kms.ScheduleKeyDeletionInput,
		optFns ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
	CreateAlias(ctx context.Context,
		params *kms.CreateAliasInput,
		optFns ...func(*kms.Options)) (*kms.CreateAliasOutput, error)
	UpdateAlias(ctx context.Context,
		params *kms.UpdateAliasInput,
		optFns ...func(*kms.Options)) (*kms.UpdateAliasOutput, error)
}

// check if the Client implements the kmsClient interface
var _ kmsClient = (*kms.Client)(nil)

// Client  represents the AWS KMS client
type Client struct {
	internalClient kmsClient
}

// Errors defines the errors that can be returned by the client
var (
	ErrMissingID                          = errors.New("key ID is required")
	ErrNativeCreateKeyFailed              = errors.New("aws key creation failed")
	ErrNativeCreateKeyFailedWrongKeyInput = errors.New(
		"aws key creation failed - wrong key input",
	)
	ErrNativeCreateAliasFailed      = errors.New("aws alias creation failed")
	ErrNativeUpdateAliasFailed      = errors.New("aws alias update failed")
	ErrNativeDeleteKeyVersionFailed = errors.New("aws key deletion failed")
	ErrEnabledKeyFailed             = errors.New("aws key enable failed")
	ErrDisabledKeyFailed            = errors.New("aws key disable failed")
	ErrUnknownOptionType            = errors.New("unknown option type")
)

// newAwsConfig creates a new AWS config with the provided loadConfigOptions
func newAwsConfig(
	ctx context.Context,
	loadConfigOptions ...func(*config.LoadOptions) error,
) aws.Config {
	return must.NotReturnError(config.LoadDefaultConfig(ctx, loadConfigOptions...))
}

// NewClientWithOptions creates a new AWS KMS client with the provided region and credentials provider.
func NewClientWithOptions(
	ctx context.Context,
	region string,
	credentialsProvider aws.CredentialsProvider,
	options ...func(*kms.Options),
) *Client {
	loadOptions := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithCredentialsProvider(credentialsProvider),
	}

	fromConfig := kms.NewFromConfig(newAwsConfig(ctx, loadOptions...), options...)

	return &Client{internalClient: fromConfig}
}

// NewClient creates a new AWS KMS client with the provided region, accessKeyID, and secretAccessKey.
// It also accepts an optional sessionToken.
func NewClient(
	ctx context.Context,
	region, accessKeyID, secretAccessKey string,
	sessionToken ...string,
) *Client {
	var token string
	if len(sessionToken) > 0 {
		token = sessionToken[0]
	}

	return NewClientFromCredentialsProvider(
		ctx,
		region,
		credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, token),
	)
}

// NewClientFromCredentialsProvider creates a new AWS KMS client with the provided credentials provider.
func NewClientFromCredentialsProvider(
	ctx context.Context,
	region string,
	credentialsProvider aws.CredentialsProvider,
) *Client {
	return NewClientWithOptions(ctx, region, credentialsProvider)
}

// NewBaseEndpointClient creates a new AWS KMS client with the provided baseEndpoint.
func NewBaseEndpointClient(
	ctx context.Context,
	region, baseEndpoint string,
) *Client {
	return NewClientWithOptions(
		ctx,
		region,
		credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
		BaseEndpoint(baseEndpoint),
	)
}

// createKeyInputFromKey converts providers.KeyInput to kms.CreateKeyInput
func createKeyInputFromKeyOptions(
	key providers.KeyInput,
) (*kms.CreateKeyInput, error) {
	var keySpec types.KeySpec

	switch key.KeyType {
	case providers.AES256:
		keySpec = types.KeySpecSymmetricDefault
	case providers.RSA3072:
		keySpec = types.KeySpecRsa3072
	case providers.RSA4096:
		keySpec = types.KeySpecRsa4096
	default:
		return nil, fmt.Errorf("%w: %v", ErrUnknownOptionType, key.KeyType)
	}

	createKeyInput := &kms.CreateKeyInput{
		KeySpec:  keySpec,
		KeyUsage: types.KeyUsageTypeEncryptDecrypt,
		Origin:   types.OriginTypeAwsKms,
	}

	return createKeyInput, nil
}

// prepareAlias - returns the alias name for the key with the given id in the format "alias/{id}-primary"
func prepareAlias(id string) string {
	return fmt.Sprintf("alias/%s-primary", id)
}

// CreateKeyVersion creates a new key version with the given keyInput
func (c *Client) CreateKeyVersion(
	ctx context.Context,
	keyInput providers.KeyInput,
) (*string, error) {
	if keyInput.ID == nil {
		return nil, ErrMissingID
	}

	createKeyInput, err := createKeyInputFromKeyOptions(keyInput)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNativeCreateKeyFailedWrongKeyInput, err)
	}

	result, err := c.internalClient.CreateKey(ctx, createKeyInput)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNativeCreateKeyFailed, err)
	}

	arn := result.KeyMetadata.Arn

	err = c.ensureAlias(ctx, *arn, *keyInput.ID)
	if err != nil {
		return nil, err
	}

	return arn, nil
}

// DeleteKeyVersion deletes the key version with the given keyID
func (c *Client) DeleteKeyVersion(
	ctx context.Context,
	keyID string,
	options providers.DeleteOptions) error {
	input := createScheduleKeyDeletionInputFromDeleteOptions(keyID, options)

	_, err := c.internalClient.ScheduleKeyDeletion(ctx, input)
	if err != nil {
		var kmsInvalidStateException *types.KMSInvalidStateException
		if errors.As(err, &kmsInvalidStateException) {
			stateErr := providers.InvalidStateError{Message: "key is probably already deleted"}
			return errs.Wrap(&stateErr, err)
		}

		return fmt.Errorf("%w: %w", ErrNativeDeleteKeyVersionFailed, err)
	}

	return nil
}

// EnableKeyVersion enables the key version with the given keyID
func (c *Client) EnableKeyVersion(ctx context.Context, keyID string) error {
	_, err := c.internalClient.EnableKey(ctx, &kms.EnableKeyInput{KeyId: ptr.PointTo(keyID)})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrEnabledKeyFailed, err)
	}

	return nil
}

// DisableKeyVersion disables the key version with the given keyID
func (c *Client) DisableKeyVersion(ctx context.Context, keyID string) error {
	_, err := c.internalClient.DisableKey(
		ctx,
		&kms.DisableKeyInput{KeyId: ptr.PointTo(keyID)},
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDisabledKeyFailed, err)
	}

	return nil
}

// ensureAlias ensures that the alias for the key with the given keyID exists
func (c *Client) ensureAlias(ctx context.Context, keyID, id string) error {
	alias := prepareAlias(id)

	err := c.createAlias(ctx, keyID, alias)
	if err != nil {
		var alreadyExistsException *types.AlreadyExistsException
		if !errors.As(err, &alreadyExistsException) {
			return err
		}
	}

	return c.updateAlias(ctx, keyID, alias)
}

// createAlias creates an alias with the given aliasName for the key with the given keyID
func (c *Client) createAlias(ctx context.Context, keyID, aliasName string) error {
	_, err := c.internalClient.CreateAlias(ctx,
		&kms.CreateAliasInput{
			AliasName:   ptr.PointTo(aliasName),
			TargetKeyId: ptr.PointTo(keyID),
		})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNativeCreateAliasFailed, err)
	}

	return nil
}

// updateAlias updates the alias with the given aliasName to point to the key with the given keyID
func (c *Client) updateAlias(ctx context.Context, keyID, aliasName string) error {
	_, err := c.internalClient.UpdateAlias(ctx, &kms.UpdateAliasInput{
		AliasName:   ptr.PointTo(aliasName),
		TargetKeyId: ptr.PointTo(keyID),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNativeUpdateAliasFailed, err)
	}

	return nil
}

// createScheduleKeyDeletionInputFromDeleteOptions converts providers.DeleteOptions to kms.ScheduleKeyDeletionInput
func createScheduleKeyDeletionInputFromDeleteOptions(
	keyID string,
	options providers.DeleteOptions) *kms.ScheduleKeyDeletionInput {
	if options.Window != nil {
		return &kms.ScheduleKeyDeletionInput{
			KeyId:               &keyID,
			PendingWindowInDays: options.Window,
		}
	}

	return &kms.ScheduleKeyDeletionInput{
		KeyId: &keyID,
	}
}
