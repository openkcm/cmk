package providers

import (
	"context"
	"errors"
)

var (
	ErrDeleteKeyVersionFailed = errors.New("delete key version failed")
)

// InvalidStateError it is error that points out that action in Client cannot be executed due to the state of the key.
// For example, trying to delete a key that is already deleted.
type InvalidStateError struct {
	Message string
}

func (e *InvalidStateError) Error() string {
	return e.Message
}

// Client is the interface for native KMS.
// Any KMS providers client we intend to use must implement this interface.
// This requires wrapping an SDK client to conform to this interface.
// For instance, refer to aws.client.
type Client interface {
	CreateKeyVersion(ctx context.Context, options KeyInput) (*string, error)
	DeleteKeyVersion(ctx context.Context, keyID string, options DeleteOptions) error
	EnableKeyVersion(ctx context.Context, keyID string) error
	DisableKeyVersion(ctx context.Context, keyID string) error
}

// DeleteOptions holds the aws_options for delete actions.
type DeleteOptions struct {
	Window *int32 // The grace period after deletion where the key material still exists in the provider
}

// KeyInput holds the aws_options for creating a key.
type KeyInput struct {
	KeyType KeyAlgorithm
	ID      *string
}
