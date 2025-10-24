package mock

import (
	"context"

	"github.com/openkcm/cmk-core/providers"
)

// Type definitions for the functions used in Client

type CreateNativeKeyFuncType func(ctx context.Context, options providers.KeyInput) (*string, error)

type DeleteNativeFuncType func(ctx context.Context, nativeKeyID string, options providers.DeleteOptions) error
type EnableNativeFuncType func(ctx context.Context, nativeKeyID string) error
type DisableNativeFuncType func(ctx context.Context, nativeKeyID string) error

// Client is a manual mock for the Client interface
type Client struct {
	CreateNativeKeyFunc CreateNativeKeyFuncType
	DeleteNativeFunc    DeleteNativeFuncType
	EnableNativeFunc    EnableNativeFuncType
	DisableNativeFunc   DisableNativeFuncType
}

// NewClientMock creates and returns a new instance of Client.
func NewClientMock() *Client {
	return &Client{}
}

// CreateKeyVersion simulates the creation of a key version by calling the CreateNativeKeyFunc.
func (m *Client) CreateKeyVersion(
	ctx context.Context,
	options providers.KeyInput,
) (*string, error) {
	return m.CreateNativeKeyFunc(ctx, options)
}

// DeleteKeyVersion simulates the deletion of a key version by calling the DeleteNativeFunc.
func (m *Client) DeleteKeyVersion(
	ctx context.Context,
	nativeKeyID string,
	options providers.DeleteOptions,
) error {
	return m.DeleteNativeFunc(ctx, nativeKeyID, options)
}

// EnableKeyVersion simulates enabling a key version by calling the EnableNativeFunc.
func (m *Client) EnableKeyVersion(ctx context.Context, nativeKeyID string) error {
	return m.EnableNativeFunc(ctx, nativeKeyID)
}

// DisableKeyVersion simulates disabling a key version by calling the DisableNativeFunc.
func (m *Client) DisableKeyVersion(ctx context.Context, nativeKeyID string) error {
	return m.DisableNativeFunc(ctx, nativeKeyID)
}

// WithCreateNativeKey sets the CreateNativeKeyFunc for the Client and returns the updated Client.
func (m *Client) WithCreateNativeKey(f CreateNativeKeyFuncType) *Client {
	m.CreateNativeKeyFunc = f
	return m
}

// WithDeleteNative sets the DeleteNativeFunc for the Client and returns the updated Client.
func (m *Client) WithDeleteNative(f DeleteNativeFuncType) *Client {
	m.DeleteNativeFunc = f
	return m
}

// WithEnableNative sets the EnableNativeFunc for the Client and returns the updated Client.
func (m *Client) WithEnableNative(f EnableNativeFuncType) *Client {
	m.EnableNativeFunc = f
	return m
}

// WithDisableNative sets the DisableNativeFunc for the Client and returns the updated Client.
func (m *Client) WithDisableNative(f DisableNativeFuncType) *Client {
	m.DisableNativeFunc = f
	return m
}
