package manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-feature/go-sdk/openfeature"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/featureflags"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

const (
	byokFeatureFlag      = "enable_byok"
	hyokFeatureFlag      = "enable_hyok"
	keyRotateFeatureFlag = "detect_hyok_rotation"

	// allowBYOKFeatureGateKey is the legacy feature gate key, used when featureFlags is not configured.
	allowBYOKFeatureGateKey = "allow-byok"
)

type FeatureFlags interface {
	IsHYOKAllowed(ctx context.Context, provider string) bool
	IsKeyRotateDetectionEnabled(ctx context.Context, provider string) bool
	IsBYOKAllowed(ctx context.Context) bool
}

type featureFlags struct {
	cfg         *config.Config
	flags       featureflags.Client
	svcRegistry serviceapi.Registry
}

func NewFeatureFlags(
	cfg *config.Config,
	svcRegistry serviceapi.Registry,
	flags featureflags.Client,
) *featureFlags {
	return &featureFlags{
		cfg:         cfg,
		svcRegistry: svcRegistry,
		flags:       flags,
	}
}

// isBYOKAllowed checks whether BYOK is enabled for the default keystore provider.
// When feature flags are not configured it falls back to the legacy allow-byok feature gate.
func (f *featureFlags) IsBYOKAllowed(ctx context.Context) bool {
	if !f.isEnabled() {
		if f.cfg == nil {
			return false
		}
		return f.cfg.FeatureGates.IsFeatureEnabled(allowBYOKFeatureGateKey)
	}

	provider, ok := f.getDefaultProvider()
	if !ok {
		return false
	}

	featureFlag := fmt.Sprintf("%s_%s", byokFeatureFlag, strings.ToLower(provider))
	enabled, err := f.flags.BooleanValue(ctx, featureFlag, false, openfeature.EvaluationContext{})
	if err != nil {
		return false
	}

	return enabled
}

// IsHYOKAllowed checks whether HYOK is enabled for the given provider.
// When feature flags are not configured it returns true for backward compatibility
// (HYOK was ungated before feature flags were introduced).
func (f *featureFlags) IsHYOKAllowed(ctx context.Context, provider string) bool {
	if !f.isEnabled() {
		return true
	}

	featureFlag := fmt.Sprintf("%s_%s", hyokFeatureFlag, strings.ToLower(provider))
	enabled, err := f.flags.BooleanValue(ctx, featureFlag, false, openfeature.EvaluationContext{})
	if err != nil {
		return false
	}

	return enabled
}

// IsKeyRotateDetectionEnabled checks whether key rotation detection is enabled for the given provider.
// When feature flags are not configured it returns true for backward compatibility.
func (f *featureFlags) IsKeyRotateDetectionEnabled(ctx context.Context, provider string) bool {
	if !f.isEnabled() {
		return true
	}

	featureFlag := fmt.Sprintf("%s_%s", keyRotateFeatureFlag, strings.ToLower(provider))
	enabled, err := f.flags.BooleanValue(ctx, featureFlag, false, openfeature.EvaluationContext{})
	if err != nil {
		return false
	}

	return enabled
}

// featureFlagsConfigured reports whether feature flags are active for this
// deployment. When it returns false, callers fall back to legacy behaviour:
// HYOK ungated and BYOK governed by the allow-byok feature gate.
func (f *featureFlags) isEnabled() bool {
	return f.cfg != nil && featureflags.Configured(f.flags, f.cfg.FeatureFlags)
}

// getDefaultProvider returns the name of the plugin tagged as DEFAULT_KEYSTORE.
func (f *featureFlags) getDefaultProvider() (string, bool) {
	if f.svcRegistry == nil {
		return "", false
	}

	plugins, err := f.svcRegistry.KeyManagementList()
	if err != nil {
		return "", false
	}

	for _, p := range plugins {
		if pluginHelpers.HasTag(p.ServiceInfo().Tags(), constants.DefaultKeyStore) {
			return p.ServiceInfo().Name(), true
		}
	}

	return "", false
}
