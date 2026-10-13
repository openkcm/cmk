package featureflags_test

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/featureflags"
)

func TestNewClient(t *testing.T) {
	c := featureflags.NewClient()
	assert.NotNil(t, c)
}

func TestClientAdapter_BooleanValue(t *testing.T) {
	c := featureflags.NewClient()
	ctx := context.Background()
	evalCtx := openfeature.EvaluationContext{}

	// The noop provider always returns the default value.
	got, err := c.BooleanValue(ctx, "any-flag", true, evalCtx)
	require.NoError(t, err)
	assert.True(t, got)

	got, err = c.BooleanValue(ctx, "any-flag", false, evalCtx)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestInit_Disabled(t *testing.T) {
	cfg := commoncfg.FeatureFlags{Enabled: false}
	err := featureflags.Init(cfg)
	assert.NoError(t, err)
}

func TestInit_Enabled_EmptyFilePath(t *testing.T) {
	cfg := commoncfg.FeatureFlags{Enabled: true, FilePath: ""}
	err := featureflags.Init(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "FilePath must not be empty")
}

func TestInit_Enabled_BadFilePath(t *testing.T) {
	cfg := commoncfg.FeatureFlags{
		Enabled:  true,
		FilePath: "/nonexistent/flags.yaml",
	}
	err := featureflags.Init(cfg)
	require.Error(t, err)
}

func TestConfigured(t *testing.T) {
	client := featureflags.NewClient()

	tests := []struct {
		name   string
		client featureflags.Client
		cfg    commoncfg.FeatureFlags
		want   bool
	}{
		{
			name:   "client present and enabled",
			client: client,
			cfg:    commoncfg.FeatureFlags{Enabled: true},
			want:   true,
		},
		{
			name:   "client present but disabled",
			client: client,
			cfg:    commoncfg.FeatureFlags{Enabled: false},
			want:   false,
		},
		{
			name:   "nil client even when enabled",
			client: nil,
			cfg:    commoncfg.FeatureFlags{Enabled: true},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, featureflags.Configured(tt.client, tt.cfg))
		})
	}
}
