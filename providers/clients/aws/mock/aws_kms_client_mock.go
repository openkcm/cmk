package mock

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"

	transport "github.com/aws/smithy-go/endpoints"
)

// Types for mocking out the AWS KMS

type CreateKeyFuncType func(ctx context.Context,
	params *kms.CreateKeyInput,
	optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
type EnableKeyFuncType func(ctx context.Context,
	params *kms.EnableKeyInput,
	optFns ...func(*kms.Options)) (*kms.EnableKeyOutput, error)
type DisableKeyFuncType func(ctx context.Context,
	params *kms.DisableKeyInput,
	optFns ...func(*kms.Options)) (*kms.DisableKeyOutput, error)
type ScheduleKeyDeletionFuncType func(ctx context.Context,
	params *kms.ScheduleKeyDeletionInput,
	optFns ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
type CreateAliasFuncType func(ctx context.Context,
	params *kms.CreateAliasInput,
	optFns ...func(*kms.Options)) (*kms.CreateAliasOutput, error)
type UpdateAliasFuncType func(ctx context.Context,
	params *kms.UpdateAliasInput,
	optFns ...func(*kms.Options)) (*kms.UpdateAliasOutput, error)

// Mock is a mock of the AWS KMS client.
type Mock struct {
	CreateKeyFunc           CreateKeyFuncType
	EnableKeyFunc           EnableKeyFuncType
	DisableKeyFunc          DisableKeyFuncType
	ScheduleKeyDeletionFunc ScheduleKeyDeletionFuncType
	CreateAliasFunc         CreateAliasFuncType
	UpdateAliasFunc         UpdateAliasFuncType
}

// NewMock creates a new instance of Mock.
func NewMock() *Mock {
	return &Mock{}
}

// CreateKey calls CreateKeyFunc if set, otherwise it panics.
func (m *Mock) CreateKey(
	ctx context.Context,
	params *kms.CreateKeyInput,
	optFns ...func(*kms.Options),
) (*kms.CreateKeyOutput, error) {
	if m.CreateKeyFunc != nil {
		return m.CreateKeyFunc(ctx, params, optFns...)
	}

	panic("CreateKeyFunc not implemented")
}

// EnableKey calls EnableKeyFunc if set, otherwise it panics.
func (m *Mock) EnableKey(
	ctx context.Context,
	params *kms.EnableKeyInput,
	optFns ...func(*kms.Options),
) (*kms.EnableKeyOutput, error) {
	if m.EnableKeyFunc != nil {
		return m.EnableKeyFunc(ctx, params, optFns...)
	}

	panic("EnableKeyFunc not implemented")
}

// DisableKey calls DisableKeyFunc if set, otherwise it panics.
func (m *Mock) DisableKey(
	ctx context.Context,
	params *kms.DisableKeyInput,
	optFns ...func(*kms.Options),
) (*kms.DisableKeyOutput, error) {
	if m.DisableKeyFunc != nil {
		return m.DisableKeyFunc(ctx, params, optFns...)
	}

	panic("DisableKeyFunc not implemented")
}

// ScheduleKeyDeletion calls ScheduleKeyDeletionFunc if set, otherwise it panics.
func (m *Mock) ScheduleKeyDeletion(
	ctx context.Context,
	params *kms.ScheduleKeyDeletionInput,
	optFns ...func(*kms.Options),
) (*kms.ScheduleKeyDeletionOutput, error) {
	if m.ScheduleKeyDeletionFunc != nil {
		return m.ScheduleKeyDeletionFunc(ctx, params, optFns...)
	}

	panic("ScheduleKeyDeletionFunc not implemented")
}

// CreateAlias calls CreateAliasFunc if set, otherwise it panics.
func (m *Mock) CreateAlias(
	ctx context.Context,
	params *kms.CreateAliasInput,
	optFns ...func(*kms.Options),
) (*kms.CreateAliasOutput, error) {
	if m.CreateAliasFunc != nil {
		return m.CreateAliasFunc(ctx, params, optFns...)
	}

	panic("CreateAliasFunc not implemented")
}

// UpdateAlias calls UpdateAliasFunc if set, otherwise it panics.
func (m *Mock) UpdateAlias(
	ctx context.Context,
	params *kms.UpdateAliasInput,
	optFns ...func(*kms.Options),
) (*kms.UpdateAliasOutput, error) {
	if m.UpdateAliasFunc != nil {
		return m.UpdateAliasFunc(ctx, params, optFns...)
	}

	panic("UpdateAliasFunc not implemented")
}

// WithCreateKeyFunc sets the CreateKeyFunc for the mock.
func (m *Mock) WithCreateKeyFunc(f CreateKeyFuncType) *Mock {
	m.CreateKeyFunc = f

	return m
}

// WithEnableKeyFunc sets the EnableKeyFunc for the mock.
func (m *Mock) WithEnableKeyFunc(f EnableKeyFuncType) *Mock {
	m.EnableKeyFunc = f

	return m
}

// WithDisableKeyFunc sets the DisableKeyFunc for the mock.
func (m *Mock) WithDisableKeyFunc(f DisableKeyFuncType) *Mock {
	m.DisableKeyFunc = f

	return m
}

// WithScheduleKeyDeletionFunc sets the ScheduleKeyDeletionFunc for the mock.
func (m *Mock) WithScheduleKeyDeletionFunc(f ScheduleKeyDeletionFuncType) *Mock {
	m.ScheduleKeyDeletionFunc = f

	return m
}

// WithCreateAliasFunc sets the CreateAliasFunc for the mock.
func (m *Mock) WithCreateAliasFunc(f CreateAliasFuncType) *Mock {
	m.CreateAliasFunc = f

	return m
}

// WithUpdateAliasFunc sets the UpdateAliasFunc for the mock.
func (m *Mock) WithUpdateAliasFunc(f UpdateAliasFuncType) *Mock {
	m.UpdateAliasFunc = f

	return m
}

// HappyPathMock creates a new instance of Mock with all methods returning success.
func HappyPathMock(expectedKeyID string, expectedKeyArn string, time time.Time) *Mock {
	return NewMock().WithCreateKeyFunc(
		func(_ context.Context,
			_ *kms.CreateKeyInput,
			_ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
			return &kms.CreateKeyOutput{KeyMetadata: &types.KeyMetadata{
				Arn:          &expectedKeyArn,
				KeyId:        &expectedKeyID,
				Enabled:      true,
				CreationDate: &time,
				KeySpec:      types.KeySpecSymmetricDefault,
			}}, nil
		}).WithCreateAliasFunc(
		func(_ context.Context,
			_ *kms.CreateAliasInput,
			_ ...func(*kms.Options)) (*kms.CreateAliasOutput, error) {
			return &kms.CreateAliasOutput{}, nil
		}).WithUpdateAliasFunc(
		func(_ context.Context, _ *kms.UpdateAliasInput, _ ...func(*kms.Options)) (*kms.UpdateAliasOutput, error) {
			return &kms.UpdateAliasOutput{}, nil
		}).
		WithEnableKeyFunc(
			func(_ context.Context,
				_ *kms.EnableKeyInput,
				_ ...func(*kms.Options)) (*kms.EnableKeyOutput, error) {
				return &kms.EnableKeyOutput{}, nil
			}).
		WithDisableKeyFunc(
			func(_ context.Context, _ *kms.DisableKeyInput, _ ...func(*kms.Options)) (*kms.DisableKeyOutput, error) {
				return &kms.DisableKeyOutput{}, nil
			},
		).
		WithScheduleKeyDeletionFunc(
			func(_ context.Context,
				_ *kms.ScheduleKeyDeletionInput,
				_ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
				return &kms.ScheduleKeyDeletionOutput{}, nil
			})
}

// ErrorMock creates a new instance of Mock with all methods returning an error.
func ErrorMock(errForced error) *Mock {
	return NewMock().WithCreateKeyFunc(
		func(_ context.Context,
			_ *kms.CreateKeyInput,
			_ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
			return nil, errForced
		}).WithCreateAliasFunc(
		func(_ context.Context,
			_ *kms.CreateAliasInput,
			_ ...func(*kms.Options)) (*kms.CreateAliasOutput, error) {
			return nil, errForced
		}).WithUpdateAliasFunc(
		func(_ context.Context, _ *kms.UpdateAliasInput, _ ...func(*kms.Options)) (*kms.UpdateAliasOutput, error) {
			return nil, errForced
		}).
		WithEnableKeyFunc(
			func(_ context.Context,
				_ *kms.EnableKeyInput,
				_ ...func(*kms.Options)) (*kms.EnableKeyOutput, error) {
				return nil, errForced
			}).
		WithDisableKeyFunc(
			func(_ context.Context, _ *kms.DisableKeyInput, _ ...func(*kms.Options)) (*kms.DisableKeyOutput, error) {
				return nil, errForced
			},
		).
		WithScheduleKeyDeletionFunc(
			func(_ context.Context,
				_ *kms.ScheduleKeyDeletionInput,
				_ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
				return nil, errForced
			})
}

// HTTPClient is a mock of the HTTPClient interface that implements the Do method.
type HTTPClient struct{}

func (c *HTTPClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK}, nil
}

// EndpointResolver is a mock of the EndpointResolver interface that implements the ResolveEndpoint method.
type EndpointResolver struct{}

func (c EndpointResolver) ResolveEndpoint(
	context.Context,
	kms.EndpointParameters,
) (transport.Endpoint, error) {
	return transport.Endpoint{}, nil
}

// CredentialsProvider is a mock of the CredentialsProvider interface that implements the Retrieve method.
type CredentialsProvider struct{}

// Retrieve returns mock credentials.
func (m *CredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "mockAccessKeyID",
		SecretAccessKey: "mockSecretAccessKey",
	}, nil
}
