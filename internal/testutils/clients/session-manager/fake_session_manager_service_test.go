package sessionmanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"

	sessionmanager "github.com/openkcm/cmk/internal/testutils/clients/session-manager"
)

var errApplyFailed = errors.New("apply failed")
var errRemoveFailed = errors.New("remove failed")

func TestFakeSessionManagerService_ApplyTrustMapping(t *testing.T) {
	ctx := t.Context()
	req := trustmappingv1.ApplyTrustMappingRequest_builder{}.Build()

	t.Run("defaults to success", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()

		resp, err := f.ApplyTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("returns the configured error", func(t *testing.T) {
		wantErr := errApplyFailed
		f := sessionmanager.NewFakeSessionManagerService()
		f.ApplyTrustMappingError = wantErr

		resp, err := f.ApplyTrustMapping(ctx, req)

		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, resp)
	})

	t.Run("includes message and success flag when configured", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()
		f.ApplyTrustMappingSuccess = false
		f.ApplyTrustMappingMessage = "could not apply"

		resp, err := f.ApplyTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.False(t, resp.GetSuccess())
		assert.Equal(t, "could not apply", resp.GetMessage())
	})
}

func TestFakeSessionManagerService_RemoveTrustMapping(t *testing.T) {
	ctx := t.Context()
	req := trustmappingv1.RemoveTrustMappingRequest_builder{}.Build()

	t.Run("defaults to success", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()

		resp, err := f.RemoveTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("returns the configured error", func(t *testing.T) {
		wantErr := errRemoveFailed
		f := sessionmanager.NewFakeSessionManagerService()
		f.RemoveTrustMappingError = wantErr

		resp, err := f.RemoveTrustMapping(ctx, req)

		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, resp)
	})

	t.Run("includes message and success flag when configured", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()
		f.RemoveTrustMappingSuccess = false
		f.RemoveTrustMappingMessage = "could not remove"

		resp, err := f.RemoveTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.False(t, resp.GetSuccess())
		assert.Equal(t, "could not remove", resp.GetMessage())
	})
}

func TestFakeSessionManagerService_BlockTrustMapping(t *testing.T) {
	ctx := t.Context()
	req := trustmappingv1.BlockTrustMappingRequest_builder{}.Build()

	t.Run("returns an empty response without a mock", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()

		resp, err := f.BlockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("delegates to the configured mock", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()
		called := false
		f.MockBlockTrustMapping = func(_ context.Context, _ *trustmappingv1.BlockTrustMappingRequest) (*trustmappingv1.BlockTrustMappingResponse, error) {
			called = true
			return trustmappingv1.BlockTrustMappingResponse_builder{Success: true}.Build(), nil
		}

		resp, err := f.BlockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.True(t, called)
		assert.True(t, resp.GetSuccess())
	})
}

func TestFakeSessionManagerService_UnblockTrustMapping(t *testing.T) {
	ctx := t.Context()
	req := trustmappingv1.UnblockTrustMappingRequest_builder{}.Build()

	t.Run("returns an empty response without a mock", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()

		resp, err := f.UnblockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("delegates to the configured mock", func(t *testing.T) {
		f := sessionmanager.NewFakeSessionManagerService()
		called := false
		f.MockUnblockTrustMapping = func(_ context.Context, _ *trustmappingv1.UnblockTrustMappingRequest) (*trustmappingv1.UnblockTrustMappingResponse, error) {
			called = true
			return trustmappingv1.UnblockTrustMappingResponse_builder{Success: true}.Build(), nil
		}

		resp, err := f.UnblockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.True(t, called)
		assert.True(t, resp.GetSuccess())
	})
}
