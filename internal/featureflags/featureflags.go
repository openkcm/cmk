package featureflags

import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/samber/oops"

	sdkfeatureflags "github.com/openkcm/common-sdk/pkg/featureflags"
)

const clientName = "cmk"

// Client abstracts OpenFeature boolean flag evaluation.
type Client interface {
	BooleanValue(
		ctx context.Context, flag string, defaultValue bool, evalCtx openfeature.EvaluationContext,
	) (bool, error)
}

// clientAdapter wraps *openfeature.Client to implement the Client interface.
type clientAdapter struct{ c *openfeature.Client }

func (a *clientAdapter) BooleanValue(
	ctx context.Context,
	flag string,
	defaultValue bool,
	evalCtx openfeature.EvaluationContext,
) (bool, error) {
	return a.c.BooleanValue(ctx, flag, defaultValue, evalCtx)
}

// NewClient returns a Client backed by the global OpenFeature provider.
func NewClient() Client {
	return &clientAdapter{c: openfeature.NewClient(clientName)}
}

// Init registers the EmbeddedProvider as the global OpenFeature provider when
// cfg.Enabled is true. It is a no-op when feature flags are disabled.
func Init(cfg commoncfg.FeatureFlags) error {
	if !cfg.Enabled {
		return nil
	}

	provider, err := sdkfeatureflags.NewEmbeddedProvider(cfg)
	if err != nil {
		return oops.Wrapf(err, "failed to create feature flag provider")
	}

	err = openfeature.SetProviderAndWait(provider)
	if err != nil {
		return oops.Wrapf(err, "failed to initialise feature flag provider")
	}
	return nil
}

// Configured reports whether feature flags are active for this deployment: a
// non-nil client is present AND the deployment enabled feature flags. It mirrors
// the guard in Init.
//
// This matters because NewClient always returns a non-nil client, even when no
// provider is registered. Callers must check the config flag too before trusting
// flag lookups; otherwise every lookup resolves to its default value.
func Configured(client Client, cfg commoncfg.FeatureFlags) bool {
	return client != nil && cfg.Enabled
}
