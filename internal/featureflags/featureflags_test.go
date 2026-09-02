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
