package manager_test

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

// rawFlagClient implements featureflags.Client for tests.
// values maps full flag keys (e.g. "enable_hyok_test") to booleans.
type rawFlagClient struct{ values map[string]bool }

func (s *rawFlagClient) BooleanValue(_ context.Context, flag string, defaultValue bool, _ openfeature.EvaluationContext) (bool, error) {
	v, ok := s.values[flag]
	if !ok {
		return defaultValue, nil
	}
	return v, nil
}

// newTestFlags returns a manager.FeatureFlags backed by NewFeatureFlags so that
// the isEnabled() guard (cfg.FeatureFlags.Enabled) is respected correctly.
func newTestFlags(cfg *config.Config, svcRegistry serviceapi.Registry, flags map[string]bool) manager.FeatureFlags {
	return manager.NewFeatureFlags(cfg, svcRegistry, &rawFlagClient{values: flags})
}

// flagsEnabledCfg returns a config with feature flags enabled, so the manager
// evaluates injected flag values instead of falling back to legacy behaviour.
func flagsEnabledCfg() *config.Config {
	c := &config.Config{}
	c.FeatureFlags.Enabled = true
	return c
}

func TestGetTenantConfigsHyokKeystore(t *testing.T) {
	tests := []struct {
		name           string
		expectedOutput []string
		enabledPlugins bool
		flags          map[string]bool
	}{
		{
			name:           "Success - One HYOK provider with flag enabled",
			expectedOutput: []string{"TEST"},
			enabledPlugins: true,
			flags:          map[string]bool{"enable_hyok_test": true},
		},
		{
			name:           "Success - HYOK plugin present but flag disabled",
			expectedOutput: []string{},
			enabledPlugins: true,
			flags:          map[string]bool{},
		},
		{
			name:           "Success - No HYOK providers",
			expectedOutput: []string{},
			enabledPlugins: false,
			flags:          map[string]bool{"enable_hyok_test": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcRegistry := testutils.NewTestPlugins(
				testplugins.WithKeyManagement(
					testplugins.Name,
					testplugins.NewTestKeyManagement(tt.enabledPlugins, false),
				),
			)

			cfg := flagsEnabledCfg()
			mgr := manager.NewTenantConfigManager(nil, svcRegistry, cfg, nil, newTestFlags(cfg, svcRegistry, tt.flags))

			result := mgr.GetTenantConfigsHyokKeystore(t.Context())
			assert.ElementsMatch(t, tt.expectedOutput, result.Provider)
			assert.IsNonDecreasing(t, result.Provider)
		})
	}
}

// TestFeatureFlagsDisabledFallback locks in the legacy behaviour that applies when
// feature flags are not configured for the deployment (cfg.FeatureFlags.Enabled is
// false, which is the case for chart versions that predate feature-flag support even
// though featureflags.NewClient always returns a non-nil client). In that case HYOK is
// ungated and BYOK is governed solely by the allow-byok feature gate; any injected flag
// values are ignored. Once feature flags are enabled, the flags take precedence instead.
func TestFeatureFlagsDisabledFallback(t *testing.T) {
	hyokPlugin := testplugins.WithKeyManagement(testplugins.Name, testplugins.NewTestKeyManagement(true, false))

	t.Run("HYOK stays enabled even though the flag would report false", func(t *testing.T) {
		svcRegistry := testutils.NewTestPlugins(hyokPlugin)
		disabledCfg := &config.Config{}
		// Feature flags disabled → fallback; flag client reports HYOK off but must be ignored.
		m := manager.NewTenantConfigManager(nil, svcRegistry, disabledCfg, nil,
			newTestFlags(disabledCfg, svcRegistry, map[string]bool{"enable_hyok_test": false}))

		res := m.GetTenantConfigsHyokKeystore(t.Context())
		assert.True(t, res.Allow)
		assert.Contains(t, res.Provider, testplugins.Name)
	})

	t.Run("HYOK stays enabled when the flag client is nil", func(t *testing.T) {
		svcRegistry := testutils.NewTestPlugins(hyokPlugin)
		nilCfg := &config.Config{}
		m := manager.NewTenantConfigManager(nil, svcRegistry, nilCfg, nil,
			manager.NewFeatureFlags(nilCfg, svcRegistry, nil))

		res := m.GetTenantConfigsHyokKeystore(t.Context())
		assert.True(t, res.Allow)
		assert.Contains(t, res.Provider, testplugins.Name)
	})

	t.Run("BYOK follows the allow-byok feature gate, ignoring the flag", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		cfg := &config.Config{}
		cfg.FeatureGates = commoncfg.FeatureGates{"allow-byok": true}
		// Gate on, flag off: while feature flags are disabled the gate wins.
		m := manager.NewTenantConfigManager(sql.NewRepository(db), testutils.NewTestPlugins(), cfg, nil,
			newTestFlags(cfg, testutils.NewTestPlugins(), map[string]bool{"enable_byok_test": false}))

		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.True(t, res.AllowBYOK)
	})

	t.Run("BYOK disabled when the allow-byok feature gate is off, ignoring the flag", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		cfg := &config.Config{}
		cfg.FeatureGates = commoncfg.FeatureGates{"allow-byok": false}
		// Gate off, flag on: the flag must be ignored while feature flags are disabled.
		m := manager.NewTenantConfigManager(sql.NewRepository(db), testutils.NewTestPlugins(), cfg, nil,
			newTestFlags(cfg, testutils.NewTestPlugins(), map[string]bool{"enable_byok_test": true}))

		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.False(t, res.AllowBYOK)
	})

	t.Run("feature flags enabled: BYOK flag takes precedence over the allow-byok gate", func(t *testing.T) {
		_, db, tenant := SetupTenantConfigManager(t)
		cfg := flagsEnabledCfg()
		cfg.FeatureGates = commoncfg.FeatureGates{"allow-byok": true}
		// Feature flags enabled → the flag (off) wins and the legacy gate (on) is ignored.
		m := manager.NewTenantConfigManager(sql.NewRepository(db), testutils.NewTestPlugins(), cfg, nil,
			newTestFlags(cfg, testutils.NewTestPlugins(), map[string]bool{"enable_byok_test": false}))

		res, err := m.GetTenantsKeystores(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.False(t, res.AllowBYOK)
	})
}
